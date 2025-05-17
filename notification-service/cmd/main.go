package main

import (
	"cleaning-app/notification-service/internal/config"
	"cleaning-app/notification-service/internal/handler"
	"cleaning-app/notification-service/internal/repository"
	"cleaning-app/notification-service/internal/services"
	"cleaning-app/notification-service/internal/utils"
	"cleaning-app/notification-service/internal/utils/push"
	"context"
	"github.com/gin-contrib/cors"
	"log"
	"net/http"
	"time"

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
	// Подключение к Firebase Cloud Messaging (FCM)
	fcm, err := push.NewFCMClient(cfg.FirebaseCredentials)
	if err != nil {
		log.Printf("Ошибка создания FCM клиента: %v", err)
		// Continue execution without FCM functionality
		fcm = nil
	} else {
		log.Println("FCM client successfully initialized")
	}

	// 5. Инициализация слоев
	repo := repository.NewNotificationRepository(db)
	notificationService := services.NewNotificationService(repo, rdb, fcm)
	notificationHandler := handler.NewNotificationHandler(notificationService)

	// 6. Запуск подписки на Redis
	go notificationService.StartRedisSubscribers(ctx)

	// 7. Инициализация маршрутов
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://host.docker.internal:8002"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	api := router.Group("/api/notifications")
	{
		// Публичные маршруты
		api.POST("/send", notificationHandler.SendManualNotification)

		// Защищенные маршруты
		secured := api.Group("/", utils.AuthMiddleware(cfg.AuthServiceURL))
		{
			secured.GET("/", notificationHandler.GetNotifications)
			secured.PUT("/:id/read", notificationHandler.MarkAsRead)
		}
	}

	// 8. Запуск HTTP сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Println("Notification service running on port", cfg.ServerPort)
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
