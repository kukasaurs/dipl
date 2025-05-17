package main

import (
	"cleaning-app/subscription-service/internal/config"
	"cleaning-app/subscription-service/internal/handler"
	"cleaning-app/subscription-service/internal/repository"
	"cleaning-app/subscription-service/internal/services"
	"cleaning-app/subscription-service/internal/utils"
	"context"
	"github.com/gin-contrib/cors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

	// Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Подключение к MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Mongo connection failed:", err)
	}

	// Регистрация завершения работы MongoDB
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	db := mongoClient.Database("cleaning_service")

	// Инициализация слоев
	repo := repository.NewSubscriptionRepository(db)

	// Создание сервисов
	subscriptionService := services.NewSubscriptionService(repo, cfg)
	notificationService := services.NewNotificationService(cfg)

	// Создание обработчиков с инъекцией сервисов
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService)

	// Запуск фоновых задач
	notifier := services.NewNotifier(subscriptionService, notificationService)
	go notifier.Start(ctx)

	// Инициализация маршрутизатора
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://host.docker.internal:8004"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Apply auth middleware to the router
	authMiddleware := utils.AuthMiddleware(cfg.AuthServiceURL)

	api := router.Group("/api/subscriptions")
	{
		// Apply auth middleware to specific routes that need it
		api.POST("/", authMiddleware, subscriptionHandler.Create)
		api.PUT("/:id", authMiddleware, subscriptionHandler.Update)
		api.DELETE("/:id", authMiddleware, subscriptionHandler.Cancel)
		api.GET("/my", authMiddleware, subscriptionHandler.GetMy)
		api.POST("/extend/:id", authMiddleware, subscriptionHandler.Extend)
		api.GET("/", authMiddleware, subscriptionHandler.GetAll)
	}

	// Настройка HTTP сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	// Запуск сервера в горутине
	go func() {
		log.Println("Subscription service running on", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Регистрация завершения работы HTTP сервера
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Shutting down HTTP server...")
		return server.Shutdown(ctx)
	})

	// Ожидание сигналов завершения
	select {}
}
