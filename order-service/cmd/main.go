package main

import (
	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/handler"
	"cleaning-app/order-service/internal/repository"
	"cleaning-app/order-service/internal/services"
	"cleaning-app/order-service/internal/utils"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
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

	// Передаём cfg в OrderHandler для корректной отправки уведомлений
	orderHandler := handler.NewOrderHandler(orderService, rdb, cfg)

	// 5. Старт фонового кэш-рефрешера
	cacheRefresher := services.NewCacheRefresher(orderService, rdb)
	cacheRefresher.Start(ctx)

	cron := services.NewCronJobService(orderRepo, cfg)
	cron.Start(ctx)

	// 6. Настройка роутера
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.RedirectTrailingSlash = false

	authMW := utils.AuthMiddleware(cfg.AuthServiceURL)

	orders := router.Group("/orders")
	orders.Use(authMW)
	{
		orders.POST("", orderHandler.CreateOrder)
		orders.POST("/", orderHandler.CreateOrder)

		orders.GET("/my", orderHandler.GetMyOrders)
		orders.GET("/:id", orderHandler.GetOrderByIDHTTP)
		orders.PUT("/:id", orderHandler.UpdateOrder)
		orders.DELETE("/:id", orderHandler.DeleteOrder)

		protected := orders.Group("/")
		protected.Use(utils.RequireRoles("manager", "admin"))
		{
			protected.PUT("/:id/assign", orderHandler.AssignCleaner)           // body: { "cleaner_id": "..." }
			protected.PUT("/:id/assign-multiple", orderHandler.AssignCleaners) // body: { "cleaner_ids": ["id1","id2"] }
			protected.PUT("/:id/unassign", orderHandler.UnassignCleaner)       // body: { "cleaner_id": "..." }

			protected.GET("/all", orderHandler.GetAllOrders)
			protected.GET("/filter", orderHandler.FilterOrders)
			protected.GET("/stats", orderHandler.GetActiveOrdersCount)
			protected.GET("/total-revenue", orderHandler.GetTotalRevenue)
		}

		protectedCleaner := orders.Group("/")
		protectedCleaner.Use(utils.RequireRoles("cleaner"))
		{
			protectedCleaner.PUT("/:id/confirm", orderHandler.ConfirmCompletion)
		}
	}
	router.POST("/api/internal/payments/notify", orderHandler.HandlePaymentNotification)

	// 7. Запуск сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Println("Order service running on", cfg.ServerPort)
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
