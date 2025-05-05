package services

import (
	"cleaning-app/order-service/internal/models"
	"cleaning-app/order-service/internal/repository"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"time"
)

type NotificationService interface {
	SendOrderNotification(ctx context.Context, order models.Order, event string) error
}

type OrderService interface {
	CreateOrder(ctx context.Context, order *models.Order) error
	UpdateOrder(ctx context.Context, id primitive.ObjectID, updated *models.Order) error
	DeleteOrder(ctx context.Context, id primitive.ObjectID) error
	AssignCleaner(ctx context.Context, id primitive.ObjectID, cleanerID string) error
	UnassignCleaner(ctx context.Context, id primitive.ObjectID) error
	ConfirmCompletion(ctx context.Context, id primitive.ObjectID, photoURL string) error
	GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetAllOrders(ctx context.Context) ([]models.Order, error)
	GetOrdersByClient(ctx context.Context, clientID string) ([]models.Order, error)
	FilterOrders(ctx context.Context, filter map[string]interface{}) ([]models.Order, error)
	RejectOrder(ctx context.Context, id primitive.ObjectID, id2 string) error
}

type orderService struct {
	repo     repository.OrderRepository
	notifier NotificationService
	redis    *redis.Client
}

func NewOrderService(repo repository.OrderRepository, notifier NotificationService, redis *redis.Client) OrderService {
	return &orderService{repo, notifier, redis}
}

func (s *orderService) CreateOrder(ctx context.Context, order *models.Order) error {
	if err := order.Validate(); err != nil {
		return err
	}
	order.Status = models.StatusPending
	if err := s.repo.Create(ctx, order); err != nil {
		return err
	}

	_ = s.redis.Del(ctx, fmt.Sprintf("orders_by_client:%s", order.ClientID)).Err()
	if err := s.redis.Del(ctx, fmt.Sprintf("orders_by_client:%s", order.ClientID)).Err(); err != nil {
		log.Printf("Failed to invalidate cache: %v", err)
	}
	return s.notifier.SendOrderNotification(ctx, *order, "created")

}

func (s *orderService) UpdateOrder(ctx context.Context, id primitive.ObjectID, updated *models.Order) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	existing.Address = updated.Address
	existing.ServiceType = updated.ServiceType
	existing.Date = updated.Date
	existing.Comment = updated.Comment

	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("orders_by_client:%s", existing.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()
	if err := s.redis.Del(ctx, cacheKey).Err(); err != nil {
		log.Printf("Failed to invalidate cache: %v", err)
	}

	return s.notifier.SendOrderNotification(ctx, *existing, "updated")
}

func (s *orderService) DeleteOrder(ctx context.Context, id primitive.ObjectID) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("orders_by_client:%s", order.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()
	if err := s.redis.Del(ctx, cacheKey).Err(); err != nil {
		log.Printf("Failed to invalidate cache: %v", err)
	}

	return s.notifier.SendOrderNotification(ctx, *order, "deleted")
}

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

	cacheKey := fmt.Sprintf("orders_by_client:%s", order.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()
	if err := s.redis.Del(ctx, cacheKey).Err(); err != nil {
		log.Printf("Failed to invalidate cache: %v", err)
	}

	return s.repo.Update(ctx, order)
}

func (s *orderService) UnassignCleaner(ctx context.Context, id primitive.ObjectID) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	order.CleanerID = nil
	order.Status = models.StatusPending

	cacheKey := fmt.Sprintf("orders_by_client:%s", order.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()

	return s.repo.Update(ctx, order)
}

func (s *orderService) ConfirmCompletion(ctx context.Context, id primitive.ObjectID, photoURL string) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	order.Status = models.StatusCompleted
	order.PhotoURL = &photoURL

	cacheKey := fmt.Sprintf("orders_by_client:%s", order.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()

	return s.repo.Update(ctx, order)
}

func (s *orderService) GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *orderService) GetAllOrders(ctx context.Context) ([]models.Order, error) {
	cacheKey := "all_orders"

	var cached []models.Order
	val, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		if jsonErr := json.Unmarshal([]byte(val), &cached); jsonErr == nil {
			return cached, nil
		}
	}

	orders, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(orders)
	_ = s.redis.Set(ctx, cacheKey, data, 5*time.Minute).Err()

	return orders, nil
}

func (s *orderService) GetOrdersByClient(ctx context.Context, clientID string) ([]models.Order, error) {
	cacheKey := fmt.Sprintf("orders_by_client:%s", clientID)

	// Пробуем получить из кэша
	var cached []models.Order
	val, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Успешно найдено в кэше
		if jsonErr := json.Unmarshal([]byte(val), &cached); jsonErr == nil {
			return cached, nil
		}
	}

	// Если в кэше нет, грузим из Mongo
	orders, err := s.repo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	// Кладём в кэш на 5 минут
	data, _ := json.Marshal(orders)
	_ = s.redis.Set(ctx, cacheKey, data, 5*time.Minute).Err()

	return orders, nil
}

func (s *orderService) FilterOrders(ctx context.Context, filter map[string]interface{}) ([]models.Order, error) {
	filterJSON, _ := json.Marshal(filter)
	hash := sha1.Sum(filterJSON)
	filterHash := hex.EncodeToString(hash[:])

	cacheKey := fmt.Sprintf("orders_filter:%s", filterHash)

	var cached []models.Order
	val, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		if jsonErr := json.Unmarshal([]byte(val), &cached); jsonErr == nil {
			return cached, nil
		}
	}

	mongoFilter := make(map[string]interface{})
	for k, v := range filter {
		mongoFilter[k] = v
	}
	orders, err := s.repo.Filter(ctx, mongoFilter)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(orders)
	_ = s.redis.Set(ctx, cacheKey, data, 5*time.Minute).Err()

	return orders, nil
}
func (s *orderService) RejectOrder(ctx context.Context, id primitive.ObjectID, cleanerID string) error {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if order.CleanerID == nil || *order.CleanerID != cleanerID {
		return errors.New("order not assigned to this cleaner")
	}

	order.CleanerID = nil
	order.Status = models.StatusPending

	if err := s.repo.Update(ctx, order); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("orders_by_client:%s", order.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()

	return s.notifier.SendOrderNotification(ctx, *order, "rejected")
}
