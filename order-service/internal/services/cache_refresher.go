package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheRefresher struct {
	orderService OrderService
	redis        *redis.Client
}

func NewCacheRefresher(orderService OrderService, redis *redis.Client) *CacheRefresher {
	return &CacheRefresher{
		orderService: orderService,
		redis:        redis,
	}
}

func (cr *CacheRefresher) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // обновлять каждые 5 минут
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

	err = cr.redis.Set(ctx, "all_orders", data, 10*time.Minute).Err()
	if err != nil {
		log.Printf("[CACHE] Failed to set all_orders cache: %v", err)
		return
	}

	log.Println("[CACHE] Successfully refreshed all_orders cache.")
}
