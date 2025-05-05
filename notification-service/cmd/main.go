package main

import (
	"context"
	"log"
	"net/http"

	"cleaning-app/notification-service/internal/config"
	"cleaning-app/notification-service/internal/handler"
	"cleaning-app/notification-service/internal/repository"
	"cleaning-app/notification-service/internal/services"
	"cleaning-app/notification-service/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// 1. Контекст и shutdown-менеджер
	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

	// 2. Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 3. Подключение к MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	db := mongoClient.Database("cleaning_service")

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// 4. Подключение к Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal("Invalid Redis URL:", err)
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to ping Redis:", err)
	}

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing Redis connection...")
		return rdb.Close()
	})

	// 5. Инициализация слоев
	repo := repository.NewNotificationRepository(db)
	notificationService := services.NewNotificationService(repo)
	notificationHandler := handler.NewNotificationHandler(notificationService)

	// 6. Инициализация маршрутов
	router := gin.Default()
	router.Use(utils.AuthMiddleware(cfg.AuthServiceURL))

	api := router.Group("/api/notifications")
	{
		api.GET("/", notificationHandler.GetNotifications)
		api.PUT("/:id/read", notificationHandler.MarkAsRead)
	}

	// 7. Запуск HTTP сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Println("Notification service running on: 8002")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Shutting down HTTP server...")
		return server.Shutdown(ctx)
	})

	// Ожидаем завершения
	select {}
}
