package services

import (
	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/models"
	"cleaning-app/order-service/internal/repository"
	"cleaning-app/order-service/internal/utils"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"time"
)

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
	GetActiveOrdersCount(ctx context.Context) (int64, error)
	GetTotalRevenue(ctx context.Context) (float64, error)
}

type orderService struct {
	repo  repository.OrderRepository
	redis *redis.Client
	cfg   *config.Config
}

func NewOrderService(repo repository.OrderRepository, redis *redis.Client, cfg *config.Config) OrderService {
	return &orderService{repo, redis, cfg}
}

func (s *orderService) CreateOrder(ctx context.Context, order *models.Order) error {
	if err := order.Validate(); err != nil {
		return err
	}
	services, err := utils.FetchServiceDetails(ctx, s.cfg.CleaningDetailsURL, order.ServiceIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch services: %w", err)
	}
	order.ServiceDetails = services

	order.Status = models.StatusPending
	if err := s.repo.Create(ctx, order); err != nil {
		return err
	}
	_ = s.redis.Del(ctx, fmt.Sprintf("orders_by_client:%s", order.ClientID)).Err()
	if err := s.redis.Del(ctx, fmt.Sprintf("orders_by_client:%s", order.ClientID)).Err(); err != nil {
		log.Printf("Failed to invalidate cache: %v", err)
	}

	managers, err := utils.GetManagers(ctx, s.cfg.AuthServiceURL)
	if err != nil {
		log.Printf("[NOTIFY] Ошибка получения менеджеров: %v\n", err)
	} else {
		for _, m := range managers {
			_ = utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
				UserID:       m.ID,
				Role:         "manager",
				Title:        "Новый заказ",
				Message:      "Создан новый заказ. Назначьте клинера.",
				Type:         "new_order",
				DeliveryType: "push",
			})
		}
	}

	utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
		UserID:       order.ClientID,
		Role:         "user",
		Title:        "Создан заказа",
		Message:      "Детали вашего заказа можно посмотреть на странице.",
		Type:         "order_created",
		DeliveryType: "push",
	})

	return nil
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
	existing.ServiceIDs = updated.ServiceIDs

	// 🔄 Получаем актуальные данные об услугах
	s.enrichWithServiceDetails(ctx, existing)

	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("orders_by_client:%s", existing.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()

	// Уведомления (без изменений)
	utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
		UserID:       existing.ClientID,
		Role:         "user",
		Title:        "Обновление заказа",
		Message:      "Детали вашего заказа были изменены.",
		Type:         "order_updated",
		DeliveryType: "push",
	})
	if existing.CleanerID != nil {
		utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
			UserID:       *existing.CleanerID,
			Role:         "cleaner",
			Title:        "Обновление заказа",
			Message:      "Детали вашего заказа были изменены.",
			Type:         "order_updated",
			DeliveryType: "push",
		})
	}

	return nil
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

	// Уведомление об удалении заказа
	utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
		UserID:       order.ClientID,
		Role:         "user",
		Title:        "Заказ удалён",
		Message:      "Один из ваших заказов был удалён.",
		Type:         "order_deleted",
		DeliveryType: "push",
	})

	if order.CleanerID != nil {
		utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
			UserID:       *order.CleanerID,
			Role:         "cleaner",
			Title:        "Заказ удалён",
			Message:      "Один из ваших заказов был удалён.",
			Type:         "order_deleted",
			DeliveryType: "push",
		})
	}

	return nil
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

	if err := s.repo.Update(ctx, order); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("orders_by_client:%s", order.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()
	if err := s.redis.Del(ctx, cacheKey).Err(); err != nil {
		log.Printf("Failed to invalidate cache: %v", err)
	}

	// Уведомление клиенту о подтверждении заказа
	utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
		UserID:       order.ClientID,
		Role:         "user",
		Title:        "Заказ подтвержден",
		Message:      "Ваш заказ успешно подтвержден и будет выполнен в назначенное время.",
		Type:         "order_confirmed",
		DeliveryType: "push",
	})

	// Уведомление клинеру о новом заказе
	utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
		UserID:       cleanerID,
		Role:         "cleaner",
		Title:        "Новый заказ",
		Message:      "Вам назначен новый заказ. Проверьте детали в вашем профиле.",
		Type:         "assigned_order",
		DeliveryType: "push",
	})

	return nil
}

func (s *orderService) UnassignCleaner(ctx context.Context, id primitive.ObjectID) error {
	// Получаем заказ только для ClientID
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("orders_by_client:%s", order.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()

	return s.repo.UnassignCleaner(ctx, id)
}
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

	cacheKey := fmt.Sprintf("orders_by_client:%s", order.ClientID)
	_ = s.redis.Del(ctx, cacheKey).Err()

	// Уведомление клиенту о завершении уборки
	utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
		UserID:       order.ClientID,
		Role:         "user",
		Title:        "Уборка завершена",
		Message:      "Уборка успешно завершена. Пожалуйста, оцените качество!",
		Type:         "cleaning_completed",
		DeliveryType: "push",
	})

	return nil
}

func (s *orderService) GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.enrichWithServiceDetails(ctx, order)
	return order, nil
}

func (s *orderService) GetAllOrders(ctx context.Context) ([]models.Order, error) {
	cacheKey := "all_orders"

	var cached []models.Order
	val, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		if jsonErr := json.Unmarshal([]byte(val), &cached); jsonErr == nil {
			for i := range cached {
				s.enrichWithServiceDetails(ctx, &cached[i])
			}
			return cached, nil
		}
	}

	orders, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		s.enrichWithServiceDetails(ctx, &orders[i])
	}

	data, _ := json.Marshal(orders)
	_ = s.redis.Set(ctx, cacheKey, data, 5*time.Minute).Err()

	return orders, nil
}

func (s *orderService) GetOrdersByClient(ctx context.Context, clientID string) ([]models.Order, error) {
	cacheKey := fmt.Sprintf("orders_by_client:%s", clientID)

	var cached []models.Order
	val, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		if jsonErr := json.Unmarshal([]byte(val), &cached); jsonErr == nil {
			for i := range cached {
				s.enrichWithServiceDetails(ctx, &cached[i])
			}
			return cached, nil
		}
	}

	orders, err := s.repo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		s.enrichWithServiceDetails(ctx, &orders[i])
	}

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
			for i := range cached {
				s.enrichWithServiceDetails(ctx, &cached[i])
			}
			return cached, nil
		}
	}

	orders, err := s.repo.Filter(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		s.enrichWithServiceDetails(ctx, &orders[i])
	}

	data, _ := json.Marshal(orders)
	_ = s.redis.Set(ctx, cacheKey, data, 5*time.Minute).Err()

	return orders, nil
}

// SendOrderNotification реализация интерфейса NotificationService
func (s *orderService) SendOrderNotification(ctx context.Context, order models.Order, event string) error {
	switch event {
	case "created":
		// Уведомление о создании заказа
		utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
			UserID:       order.ClientID,
			Role:         "user",
			Title:        "Заказ создан",
			Message:      "Ваш заказ успешно создан и ожидает подтверждения.",
			Type:         "order_created",
			DeliveryType: "push",
		})
	case "updated":
		// Уведомление об изменении заказа
		utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
			UserID:       order.ClientID,
			Role:         "user",
			Title:        "Обновление заказа",
			Message:      "Детали вашего заказа были изменены.",
			Type:         "order_updated",
			DeliveryType: "push",
		})
		if order.CleanerID != nil {
			utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
				UserID:       *order.CleanerID,
				Role:         "cleaner",
				Title:        "Обновление заказа",
				Message:      "Детали вашего заказа были изменены.",
				Type:         "order_updated",
				DeliveryType: "push",
			})
		}
	case "deleted":
		// Уведомление об удалении заказа
		utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
			UserID:       order.ClientID,
			Role:         "user",
			Title:        "Заказ удалён",
			Message:      "Один из ваших заказов был удалён.",
			Type:         "order_deleted",
			DeliveryType: "push",
		})
		if order.CleanerID != nil {
			utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
				UserID:       *order.CleanerID,
				Role:         "cleaner",
				Title:        "Заказ удалён",
				Message:      "Один из ваших заказов был удалён.",
				Type:         "order_deleted",
				DeliveryType: "push",
			})
		}
	}
	return nil
}
func (s *orderService) enrichWithServiceDetails(ctx context.Context, order *models.Order) {
	if len(order.ServiceIDs) == 0 {
		return
	}
	services, err := utils.FetchServiceDetails(ctx, s.cfg.CleaningDetailsURL, order.ServiceIDs)
	if err == nil {
		order.ServiceDetails = services
	}
}

func (s *orderService) GetActiveOrdersCount(ctx context.Context) (int64, error) {
	filter := bson.M{"status": bson.M{"$in": []string{"new", "assigned", "in_progress"}}}
	return s.repo.CountOrders(ctx, filter)
}

func (s *orderService) GetTotalRevenue(ctx context.Context) (float64, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"status": "completed"}},
		{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": "$price"},
		}},
	}

	cursor, err := s.repo.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}

	var result []struct {
		Total float64 `bson:"total"`
	}
	if err = cursor.All(ctx, &result); err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Total, nil
}
