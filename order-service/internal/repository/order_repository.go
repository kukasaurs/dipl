package repository

import (
	"context"
	"errors"
	"time"

	"cleaning-app/order-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// orderRepository отвечает за CRUD над коллекцией "orders".
type orderRepository struct {
	collection *mongo.Collection
}

// NewOrderRepository создаёт репозиторий для заказов.
func NewOrderRepository(db *mongo.Database) *orderRepository {
	return &orderRepository{collection: db.Collection("orders")}
}

func (r *orderRepository) Create(ctx context.Context, order *models.Order) error {
	order.ID = primitive.NewObjectID()
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, order)
	return err
}

func (r *orderRepository) Update(ctx context.Context, order *models.Order) error {
	order.UpdatedAt = time.Now()
	_, err := r.collection.UpdateByID(ctx, order.ID, bson.M{"$set": order})
	return err
}

func (r *orderRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *orderRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	var order models.Order
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&order)
	return &order, err
}

func (r *orderRepository) GetByClientID(ctx context.Context, clientID string) ([]models.Order, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"client_id": clientID})
	if err != nil {
		return nil, err
	}
	var orders []models.Order
	err = cursor.All(ctx, &orders)
	return orders, err
}

func (r *orderRepository) GetAll(ctx context.Context) ([]models.Order, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var orders []models.Order
	err = cursor.All(ctx, &orders)
	return orders, err
}

func (r *orderRepository) Filter(ctx context.Context, filter bson.M) ([]models.Order, error) {
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var orders []models.Order
	err = cursor.All(ctx, &orders)
	return orders, err
}

// -------------------- НОВЫЕ МЕТОДЫ --------------------

// IsCleanerBusy проверяет, есть ли у данного клинера в указанную дату
// активные заказы (status == assigned или status == pending, можно расширить).
func (r *orderRepository) IsCleanerBusy(ctx context.Context, cleanerID string, date time.Time) (bool, error) {
	// Будем считать, что «занят» означает: в массиве cleaner_id уже есть cleanerID
	// и поле date ровно совпадает (точное время). При желании можно добавить
	// диапазон +-час, но здесь проверяем простое совпадение.
	filter := bson.M{
		"cleaner_id": cleanerID,
		"date":       date,
		"status": bson.M{
			"$in": []models.OrderStatus{models.StatusAssigned, models.StatusPending},
		},
	}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// AddCleanerToOrder добавляет одного клинера в массив поле cleaner_id,
// предварительно проверив, что он не занят на эту дату.
func (r *orderRepository) AddCleanerToOrder(ctx context.Context, orderID primitive.ObjectID, cleanerID string) error {
	// Сначала получим заказ, чтобы узнать поле date.
	order, err := r.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	// Проверяем занятость:
	busy, err := r.IsCleanerBusy(ctx, cleanerID, order.Date)
	if err != nil {
		return err
	}
	if busy {
		return errors.New("selected cleaner is busy at this date")
	}
	// Уточним, не дублируется ли уже этот cleaner:
	for _, existing := range order.CleanerID {
		if existing == cleanerID {
			return errors.New("cleaner already assigned to this order")
		}
	}
	// Добавляем в массив (MongoDB $addToSet гарантирует уникальность).
	update := bson.M{
		"$addToSet": bson.M{"cleaner_id": cleanerID},
		"$set":      bson.M{"status": models.StatusAssigned, "updated_at": time.Now()},
	}
	_, err = r.collection.UpdateByID(ctx, orderID, update)
	return err
}

// RemoveCleanerFromOrder убирает одного клинера из массива cleaner_id.
// Если после этого массив станет пустым, переводит status обратно в pending.
func (r *orderRepository) RemoveCleanerFromOrder(ctx context.Context, orderID primitive.ObjectID, cleanerID string) error {
	// Удаляем данного клинера из массива:
	update := bson.M{
		"$pull": bson.M{"cleaner_id": cleanerID},
		"$set":  bson.M{"updated_at": time.Now()},
	}
	_, err := r.collection.UpdateByID(ctx, orderID, update)
	if err != nil {
		return err
	}
	// После pull проверим, остались ли ещё клинеры в заказе:
	order, err := r.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if len(order.CleanerID) == 0 {
		// Если нет ни одного, сбрасываем статус обратно в pending
		_, err = r.collection.UpdateByID(ctx, orderID, bson.M{
			"$set": bson.M{"status": models.StatusPending, "updated_at": time.Now()},
		})
		return err
	}
	return nil
}

// -------------------------------------------------------

// UnassignCleaner раньше «удалял поле cleaner_id», теперь заменим его
// на вызов RemoveCleanerFromOrder или, если у вас изначально ожидался
// single-assignment, можно оставить для совместимости, но лучше
// использовать новый метод RemoveCleanerFromOrder в сервисе.
func (r *orderRepository) UnassignCleaner(ctx context.Context, id primitive.ObjectID) error {

	update := bson.M{
		"$unset": bson.M{"cleaner_id": ""},
		"$set":   bson.M{"status": models.StatusPending, "updated_at": time.Now()},
	}
	_, err := r.collection.UpdateByID(ctx, id, update)
	return err
}

func (r *orderRepository) CountOrders(ctx context.Context, filter interface{}) (int64, error) {
	return r.collection.CountDocuments(ctx, filter)
}

func (r *orderRepository) Aggregate(ctx context.Context, pipeline []bson.M) (*mongo.Cursor, error) {
	return r.collection.Aggregate(ctx, pipeline)
}
func (r *orderRepository) FindByCleaner(ctx context.Context, cleanerID primitive.ObjectID) ([]models.Order, error) {
	filter := bson.M{
		"cleaner_id": cleanerID.Hex(),
	}
	cursor, err := r.collection.Find(ctx, filter)

	if err != nil {
		return nil, err
	}
	var orders []models.Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

// CountCompletedByCleaner возвращает кол-во заказов со статусом "completed", в которых участвует этот cleanerID.
func (r *orderRepository) CountCompletedByCleaner(ctx context.Context, cleanerID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"cleaner_id": cleanerID.Hex(),
		"status":     models.StatusCompleted,
	}
	return r.collection.CountDocuments(ctx, filter)
}
