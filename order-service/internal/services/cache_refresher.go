package services

import (
	"cleaning-app/order-service/internal/models"
	"context"
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheRefresher struct {
	orderService OrderService
	redis        *redis.Client
}

type OrderService interface {
	clearCache(ctx context.Context, clientID string)
	CreateOrder(ctx context.Context, order *models.Order) error
	UpdateOrder(ctx context.Context, id primitive.ObjectID, updated *models.Order) error
	DeleteOrder(ctx context.Context, id primitive.ObjectID) error
	AssignCleaners(ctx context.Context, id primitive.ObjectID, cleanerIDs []string) error
	AssignCleaner(ctx context.Context, id primitive.ObjectID, cleanerID string) error
	UnassignCleaner(ctx context.Context, id primitive.ObjectID, cleanerID string) error
	ConfirmCompletion(ctx context.Context, id primitive.ObjectID, photoURL string) error
	GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetAllOrders(ctx context.Context) ([]models.Order, error)
	GetOrdersByClient(ctx context.Context, clientID string) ([]models.Order, error)
	FilterOrders(ctx context.Context, filter map[string]interface{}) ([]models.Order, error)
	GetActiveOrdersCount(ctx context.Context) (int64, error)
	enrichWithServiceDetails(ctx context.Context, order *models.Order)
	UpdatePaymentStatus(ctx context.Context, orderID string, status string) error
}

func NewCacheRefresher(orderService *orderService, redis *redis.Client) *CacheRefresher {
	return &CacheRefresher{
		orderService: orderService,
		redis:        redis,
	}
}

func (cr *CacheRefresher) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Println("[CACHE] Refreshing all caches...")
				cr.refreshAllOrdersCache(ctx)
			case <-ctx.Done():
				log.Println("[CACHE] Stopping cache refresher...")
				ticker.Stop()
				return
			}
		}
	}()
}

func (cr *CacheRefresher) refreshAllOrdersCache(ctx context.Context) {
	orders, err := cr.orderService.GetAllOrders(ctx)
	if err != nil {
		log.Printf("[CACHE] Failed to refresh all orders cache: %v", err)
		return
	}

	data, err := json.Marshal(orders)
	if err != nil {
		log.Printf("[CACHE] Failed to marshal orders: %v", err)
		return
	}

	err = cr.redis.Set(ctx, "all_orders", data, 10*time.Second).Err()
	if err != nil {
		log.Printf("[CACHE] Failed to set all_orders cache: %v", err)
		return
	}

	log.Println("[CACHE] Successfully refreshed all_orders cache.")
}
