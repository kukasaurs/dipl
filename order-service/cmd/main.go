package main

import (
	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/handler"
	"cleaning-app/order-service/internal/repository"
	"cleaning-app/order-service/internal/services"
	"cleaning-app/order-service/internal/utils"
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"time"
)

func main() {
	// 1. Базовый контекст + менеджер завершения
	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 2. Инициализация MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	db := mongoClient.Database("cleaning_service")

	// Зарегистрировать Mongo для Graceful Shutdown
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// 3. Инициализация Redis
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal("Invalid Redis URL:", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to ping Redis:", err)
	}

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing Redis connection...")
		return rdb.Close()
	})

	// 4. Инициализация сервисов
	orderRepo := repository.NewOrderRepository(db)
	orderService := services.NewOrderService(orderRepo, rdb, cfg)

	orderHandler := handler.NewOrderHandler(orderService)

	// 5. Старт фонового кэш-рефрешера
	cacheRefresher := services.NewCacheRefresher(orderService, rdb)
	cacheRefresher.Start(ctx)

	cron := services.NewCronJobService(orderRepo, cfg)
	cron.Start(ctx)

	// 6. Настройка роутера
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:8001"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.Use(utils.AuthMiddleware(cfg.AuthServiceURL))
	orders := router.Group("/api/orders")
	{
		orders.POST("/", orderHandler.CreateOrder)
		orders.GET("/my", orderHandler.GetMyOrders)
		orders.PUT("/:id", orderHandler.UpdateOrder)
		orders.DELETE("/:id", orderHandler.DeleteOrder)

		protected := orders.Group("/")
		protected.Use(utils.RequireRoles("manager", "admin"))
		{
			protected.PUT("/:id/assign", orderHandler.AssignCleaner)
			protected.PUT("/:id/unassign", orderHandler.UnassignCleaner)
			protected.GET("/all", orderHandler.GetAllOrders)
			protected.GET("/filter", orderHandler.FilterOrders)
		}

		protectedCleaner := orders.Group("/")
		protectedCleaner.Use(utils.RequireRoles("cleaner"))
		{
			protectedCleaner.PUT("/:id/confirm", orderHandler.ConfirmCompletion)
		}
	}

	// 7. Запуск сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Println("Order service running on :8001")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Регистрация graceful shutdown сервера
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Shutting down HTTP server...")
		return server.Shutdown(ctx)
	})

	// Всё остальное делает shutdownManager
	select {}
}
