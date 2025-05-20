package main

import (
	"cleaning-app/subscription-service/internal/config"
	"cleaning-app/subscription-service/internal/handler"
	"cleaning-app/subscription-service/internal/repository"
	"cleaning-app/subscription-service/internal/services"
	"cleaning-app/subscription-service/internal/utils"

	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"time"
)

func main() {
	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

	// 1. Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 2. Подключение к MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Mongo connection failed:", err)
	}
	db := mongoClient.Database("cleaning_service")

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// 3. Инициализация зависимостей
	repo := repository.NewSubscriptionRepository(db)

	orderClient := utils.NewOrderClient(cfg.OrderServiceURL)
	paymentClient := utils.NewPaymentClient(cfg.PaymentServiceURL)

	subService := services.NewSubscriptionService(repo, orderClient, paymentClient)
	notifier := services.NewNotifier(subService, nil) // если тебе нужны уведомления — передай NotificationService
	go notifier.Start(ctx)
	// 4. Обработчики
	subHandler := handler.NewSubscriptionHandler(subService)

	// 5. Cron-задача авто-заказов по подписке
	go utils.StartSubscriptionScheduler(ctx, subService.ProcessDailyOrders)

	// 6. Настройка Gin и CORS
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://host.docker.internal:8004"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 7. Middleware
	authMW := utils.AuthMiddleware(cfg.AuthServiceURL)

	api := router.Group("/api/subscriptions", authMW)
	api.Use(authMW)
	{
		api.POST("/", subHandler.Create)
		api.POST("/extend/:id", subHandler.Extend)
		api.GET("/my", subHandler.GetMy)
		api.GET("/", subHandler.GetAll)
		// Если остались:
		api.PUT("/:id", subHandler.Update)
		api.DELETE("/:id", subHandler.Cancel)
	}

	// 8. HTTP-сервер
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Println("Subscription service running on", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Shutting down HTTP server...")
		return server.Shutdown(ctx)
	})

	select {}
}
