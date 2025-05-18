package repository

import (
	"cleaning-app/order-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	Update(ctx context.Context, order *models.Order) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByClientID(ctx context.Context, clientID string) ([]models.Order, error)
	GetAll(ctx context.Context) ([]models.Order, error)
	Filter(ctx context.Context, filter bson.M) ([]models.Order, error)
	UnassignCleaner(ctx context.Context, id primitive.ObjectID) error
}

type orderRepository struct {
	collection *mongo.Collection
}

func NewOrderRepository(db *mongo.Database) OrderRepository {
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
func (r *orderRepository) UnassignCleaner(ctx context.Context, id primitive.ObjectID) error {
	update := bson.M{
		"$unset": bson.M{"cleaner_id": ""}, // Удаляем поле
		"$set": bson.M{
			"status":     models.StatusPending,
			"updated_at": time.Now(),
		},
	}
	_, err := r.collection.UpdateByID(ctx, id, update)
	return err
}
