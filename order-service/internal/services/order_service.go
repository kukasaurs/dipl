package services

import (
	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/models"
	"cleaning-app/order-service/internal/utils"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type orderService struct {
	repo  OrderRepository
	redis *redis.Client
	cfg   *config.Config
}

type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	Update(ctx context.Context, order *models.Order) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByClientID(ctx context.Context, clientID string) ([]models.Order, error)
	GetAll(ctx context.Context) ([]models.Order, error)
	Filter(ctx context.Context, filter bson.M) ([]models.Order, error)
	UnassignCleaner(ctx context.Context, id primitive.ObjectID) error
	CountOrders(ctx context.Context, filter interface{}) (int64, error)
	Aggregate(ctx context.Context, pipeline []bson.M) (*mongo.Cursor, error)
}

// NewOrderService constructs a new OrderService.
func NewOrderService(repo OrderRepository, rdb *redis.Client, cfg *config.Config) *orderService {
	return &orderService{repo: repo, redis: rdb, cfg: cfg}
}

// clearCache invalidates Redis caches for orders.
func (s *orderService) clearCache(ctx context.Context, clientID string) {
	// keys to remove
	keys := []string{
		fmt.Sprintf("orders_by_client:%s", clientID),
		"all_orders",
	}
	s.redis.Del(ctx, keys...)
	// remove any filter caches
	if fltKeys, err := s.redis.Keys(ctx, "orders_filter:*").Result(); err == nil {
		s.redis.Del(ctx, fltKeys...)
	}
}

// CreateOrder creates a new order and invalidates cache.
func (s *orderService) CreateOrder(ctx context.Context, order *models.Order) error {
	if err := order.Validate(); err != nil {
		return err
	}
	s.enrichWithServiceDetails(ctx, order)
	order.Status = models.StatusPending

	if err := s.repo.Create(ctx, order); err != nil {
		return err
	}
	// invalidate caches for this client
	s.clearCache(ctx, order.ClientID)
	// send notifications omitted for brevity
	return nil
}

// UpdateOrder updates an existing order and invalidates cache.
func (s *orderService) UpdateOrder(ctx context.Context, id primitive.ObjectID, updated *models.Order) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// присваиваем ID, иначе UpdateByID не сработает
	updated.ID = id

	// применяем обновления
	existing.Address = updated.Address
	existing.ServiceType = updated.ServiceType
	existing.Date = updated.Date
	existing.Comment = updated.Comment
	existing.ServiceIDs = updated.ServiceIDs

	s.enrichWithServiceDetails(ctx, existing)

	// обновляем
	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	s.clearCache(ctx, existing.ClientID)
	return nil
}

// DeleteOrder deletes an order and invalidates cache.
func (s *orderService) DeleteOrder(ctx context.Context, id primitive.ObjectID) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// invalidate cache
	s.clearCache(ctx, order.ClientID)
	return nil
}

// AssignCleaner assigns a cleaner and invalidates cache.
func (s *orderService) AssignCleaner(ctx context.Context, id primitive.ObjectID, cleanerID string) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if order.CleanerID != nil {
		return errors.New("cleaner already assigned")
	}
	order.CleanerID = &cleanerID
	order.Status = models.StatusAssigned
	if err := s.repo.Update(ctx, order); err != nil {
		return err
	}
	// invalidate cache
	s.clearCache(ctx, order.ClientID)
	return nil
}

// UnassignCleaner unassigns a cleaner and invalidates cache.
func (s *orderService) UnassignCleaner(ctx context.Context, id primitive.ObjectID) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.UnassignCleaner(ctx, id); err != nil {
		return err
	}
	// invalidate cache
	s.clearCache(ctx, order.ClientID)
	return nil
}

// ConfirmCompletion marks an order completed and invalidates cache.
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
	// invalidate cache
	s.clearCache(ctx, order.ClientID)
	return nil
}

// GetOrderByID retrieves a single order.
func (s *orderService) GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.enrichWithServiceDetails(ctx, order)
	return order, nil
}

// GetAllOrders returns all orders with caching.
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
	// fallback to DB
	orders, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		s.enrichWithServiceDetails(ctx, &orders[i])
	}
	// cache
	if data, err := json.Marshal(orders); err == nil {
		s.redis.Set(ctx, cacheKey, data, 30*time.Second)
	}
	return orders, nil
}

// GetOrdersByClient returns orders for a client with caching.
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

// FilterOrders filters orders by criteria with caching.
func (s *orderService) FilterOrders(ctx context.Context, filter map[string]interface{}) ([]models.Order, error) {
	// compute hash for filter
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

// GetActiveOrdersCount returns count of active orders.
func (s *orderService) GetActiveOrdersCount(ctx context.Context) (int64, error) {
	filter := bson.M{"status": bson.M{"$in": []string{"new", "assigned", "in_progress"}}}
	return s.repo.CountOrders(ctx, filter)
}

// GetTotalRevenue returns sum of completed orders.
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

// enrichWithServiceDetails populates service details and total price.
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
	// преобразуем строку в ObjectID
	id, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		return fmt.Errorf("invalid order id: %w", err)
	}
	// получаем заказ
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
