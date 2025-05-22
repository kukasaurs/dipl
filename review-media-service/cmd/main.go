package main

import (
	"cleaning-app/review-media-service/internal/config"
	"cleaning-app/review-media-service/internal/handler"
	"cleaning-app/review-media-service/internal/repository"
	"cleaning-app/review-media-service/internal/services"
	"cleaning-app/review-media-service/internal/utils"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
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

	// 2. Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 3. Инициализация MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	db := mongoClient.Database("cleaning_service")

	// Регистрация MongoDB для Graceful Shutdown
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// 4. Инициализация Minio
	minioClient, err := utils.InitMinio(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey)
	if err != nil {
		log.Fatal("Failed to initialize Minio client:", err)
	}

	// 5. Инициализация репозиториев и сервисов
	reviewRepo := repository.NewReviewRepository(db)
	mediaRepo := repository.NewMediaRepository(db)
	reviewService := services.NewReviewService(reviewRepo, cfg)
	mediaService := services.NewMediaService(mediaRepo, minioClient, cfg.BucketName, cfg)

	// 6. Инициализация обработчиков
	reviewHandler := handler.NewReviewHandler(reviewService)
	mediaHandler := handler.NewMediaHandler(mediaService)

	// 7. Настройка роутера
	router := gin.Default()

	// Аутентификация
	router.Use(utils.AuthMiddleware(cfg.AuthServiceURL))

	// Группа маршрутов для отзывов
	reviews := router.Group("/reviews")
	{
		reviews.POST("/", reviewHandler.CreateReview)
		reviews.GET("/user/:id", reviewHandler.GetReviewsByUser)
		reviews.POST("/reminder", reviewHandler.TriggerReviewReminder)
		reviews.GET("/statistics", reviewHandler.GetStatistics)
	}

	// Группа маршрутов для медиа
	media := router.Group("/media")
	{
		media.POST("/upload", mediaHandler.Upload)
		media.GET("/order/:order_id", mediaHandler.GetMediaByOrder)
	}

	// 8. Запуск сервера
	server := &http.Server{
		Addr:    "0.0.0.0:8007",
		Handler: router,
	}

	go func() {
		log.Println("Review and Media Service running on port 8007")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Регистрация graceful shutdown сервера
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Shutting down HTTP server...")
		return server.Shutdown(ctx)
	})

	// Ожидание сигналов завершения
	select {}
}
