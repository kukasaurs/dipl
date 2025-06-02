package services

import (
	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/models"
	"cleaning-app/order-service/internal/utils"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	_ "errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"

	"github.com/redis/go-redis/v9"
)

// OrderRepository изменился: теперь содержит AddCleanerToOrder, RemoveCleanerFromOrder и IsCleanerBusy.
type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	Update(ctx context.Context, order *models.Order) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByClientID(ctx context.Context, clientID string) ([]models.Order, error)
	GetAll(ctx context.Context) ([]models.Order, error)
	Filter(ctx context.Context, filter bson.M) ([]models.Order, error)
	UnassignCleaner(ctx context.Context, id primitive.ObjectID) error
	AddCleanerToOrder(ctx context.Context, orderID primitive.ObjectID, cleanerID string) error
	RemoveCleanerFromOrder(ctx context.Context, orderID primitive.ObjectID, cleanerID string) error
	IsCleanerBusy(ctx context.Context, cleanerID string, date time.Time) (bool, error)
	CountOrders(ctx context.Context, filter interface{}) (int64, error)
	Aggregate(ctx context.Context, pipeline []bson.M) (*mongo.Cursor, error)
}

type orderService struct {
	repo  OrderRepository
	redis *redis.Client
	cfg   *config.Config
}

// NewOrderService конструирует сервис заказов.
func NewOrderService(repo OrderRepository, rdb *redis.Client, cfg *config.Config) *orderService {
	return &orderService{repo: repo, redis: rdb, cfg: cfg}
}

// clearCache invalidates Redis-кэш.
func (s *orderService) clearCache(ctx context.Context, clientID string) {
	keys := []string{
		fmt.Sprintf("orders_by_client:%s", clientID),
		"all_orders",
	}
	s.redis.Del(ctx, keys...)
	if fltKeys, err := s.redis.Keys(ctx, "orders_filter:*").Result(); err == nil {
		s.redis.Del(ctx, fltKeys...)
	}
}

// CreateOrder остаётся без изменений, кроме уведомления и кэша.
func (s *orderService) CreateOrder(ctx context.Context, order *models.Order) error {
	if err := order.Validate(); err != nil {
		return err
	}
	s.enrichWithServiceDetails(ctx, order)
	order.Status = models.StatusPending

	if err := s.repo.Create(ctx, order); err != nil {
		return err
	}
	s.clearCache(ctx, order.ClientID)
	return nil
}

// UpdateOrder без изменений.
func (s *orderService) UpdateOrder(ctx context.Context, id primitive.ObjectID, updated *models.Order) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	updated.ID = id
	existing.Address = updated.Address
	existing.ServiceType = updated.ServiceType
	existing.Date = updated.Date
	existing.Comment = updated.Comment
	existing.ServiceIDs = updated.ServiceIDs

	s.enrichWithServiceDetails(ctx, existing)
	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}
	s.clearCache(ctx, existing.ClientID)
	return nil
}

// DeleteOrder без изменений.
func (s *orderService) DeleteOrder(ctx context.Context, id primitive.ObjectID) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.clearCache(ctx, order.ClientID)
	return nil
}

// ---------------- НОВЫЙ МЕТОД: AssignCleaners --------------------
// AssignCleaners принимает массив cleanerIDs и пытается добавить каждого клинера.
func (s *orderService) AssignCleaners(ctx context.Context, id primitive.ObjectID, cleanerIDs []string) error {
	// Получим сам заказ, чтобы знать дату и clientID для кэша:
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// После добавления любого клинера переведём статус в Assigned:
	for _, cleanerID := range cleanerIDs {
		// Проверяем занятость и добавляем:
		if err := s.repo.AddCleanerToOrder(ctx, id, cleanerID); err != nil {
			// Если один из клинеров занят, возвращаем ошибку и не продолжаем дальше.
			return fmt.Errorf("cannot assign cleaner %s: %w", cleanerID, err)
		}
	}

	order.Status = models.StatusAssigned
	if err := s.repo.Update(ctx, order); err != nil {
		return err
	}

	// инвалидируем кэш клиента:
	s.clearCache(ctx, order.ClientID)
	return nil
}

// ---------------- Переопределён AssignCleaner (оставлен для совместимости) ---------------
// Теперь AssignCleaner просто оборачивает AssignCleaners с одним элементом массива.
func (s *orderService) AssignCleaner(ctx context.Context, id primitive.ObjectID, cleanerID string) error {
	return s.AssignCleaners(ctx, id, []string{cleanerID})
}

// UnassignCleaner вызывает RemoveCleanerFromOrder для одного клинера.
// Если передавать empty string — можно вызвать repo.UnassignCleaner, но лучше явно убирать одного.
func (s *orderService) UnassignCleaner(ctx context.Context, id primitive.ObjectID, cleanerID string) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.RemoveCleanerFromOrder(ctx, id, cleanerID); err != nil {
		return err
	}
	// Если после удаления нет ни одного клинера, статус будет внутри репозитория переведён в pending.
	s.clearCache(ctx, order.ClientID)
	return nil
}

// ConfirmCompletion без изменений.
func (s *orderService) ConfirmCompletion(ctx context.Context, id primitive.ObjectID, photoURL string) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	order.Status = models.StatusCompleted
	order.PhotoURL = &photoURL
	if err := s.repo.Update(ctx, order); err != nil {
		return err
	}
	s.clearCache(ctx, order.ClientID)
	return nil
}

// GetOrderByID без изменений.
func (s *orderService) GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.enrichWithServiceDetails(ctx, order)
	return order, nil
}

// GetAllOrders без изменений.
func (s *orderService) GetAllOrders(ctx context.Context) ([]models.Order, error) {
	cacheKey := "all_orders"
	var result []models.Order
	if data, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
		if err := json.Unmarshal([]byte(data), &result); err == nil {
			for i := range result {
				s.enrichWithServiceDetails(ctx, &result[i])
			}
			return result, nil
		}
	}
	orders, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		s.enrichWithServiceDetails(ctx, &orders[i])
	}
	if data, err := json.Marshal(orders); err == nil {
		s.redis.Set(ctx, cacheKey, data, 30*time.Second)
	}
	return orders, nil
}

// GetOrdersByClient без изменений.
func (s *orderService) GetOrdersByClient(ctx context.Context, clientID string) ([]models.Order, error) {
	cacheKey := fmt.Sprintf("orders_by_client:%s", clientID)
	var result []models.Order
	if data, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
		if err := json.Unmarshal([]byte(data), &result); err == nil {
			for i := range result {
				s.enrichWithServiceDetails(ctx, &result[i])
			}
			return result, nil
		}
	}
	orders, err := s.repo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		s.enrichWithServiceDetails(ctx, &orders[i])
	}
	if data, err := json.Marshal(orders); err == nil {
		s.redis.Set(ctx, cacheKey, data, 5*time.Minute)
	}
	return orders, nil
}

// FilterOrders без изменений.
func (s *orderService) FilterOrders(ctx context.Context, filter map[string]interface{}) ([]models.Order, error) {
	b, _ := json.Marshal(filter)
	hash := sha1.Sum(b)
	cacheKey := fmt.Sprintf("orders_filter:%s", hex.EncodeToString(hash[:]))
	var result []models.Order
	if data, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
		if err := json.Unmarshal([]byte(data), &result); err == nil {
			for i := range result {
				s.enrichWithServiceDetails(ctx, &result[i])
			}
			return result, nil
		}
	}
	orders, err := s.repo.Filter(ctx, bson.M(filter))
	if err != nil {
		return nil, err
	}
	for i := range orders {
		s.enrichWithServiceDetails(ctx, &orders[i])
	}
	if data, err := json.Marshal(orders); err == nil {
		s.redis.Set(ctx, cacheKey, data, 5*time.Minute)
	}
	return orders, nil
}

// GetActiveOrdersCount без изменений.
func (s *orderService) GetActiveOrdersCount(ctx context.Context) (int64, error) {
	filter := bson.M{"status": bson.M{"$in": []string{string(models.StatusPending), string(models.StatusAssigned)}}}
	return s.repo.CountOrders(ctx, filter)
}

// GetTotalRevenue без изменений.
func (s *orderService) GetTotalRevenue(ctx context.Context) (float64, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"status": models.StatusCompleted}},
		{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$total_price"}}},
	}
	cursor, err := s.repo.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	var out []struct {
		Total float64 `bson:"total"`
	}
	if err := cursor.All(ctx, &out); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	return out[0].Total, nil
}

// enrichWithServiceDetails без изменений.
func (s *orderService) enrichWithServiceDetails(ctx context.Context, order *models.Order) {
	if len(order.ServiceIDs) == 0 {
		return
	}
	services, err := utils.FetchServiceDetails(ctx, s.cfg.CleaningDetailsURL, order.ServiceIDs)
	if err != nil {
		return
	}
	order.ServiceDetails = services
	total := 0.0
	for _, svc := range services {
		total += svc.Price
	}
	order.TotalPrice = total
}

func (s *orderService) UpdatePaymentStatus(ctx context.Context, orderID string, status string) error {
	id, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		return fmt.Errorf("invalid order id: %w", err)
	}
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	order.Status = models.StatusPaid
	if err := s.repo.Update(ctx, order); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}
	return nil
}
