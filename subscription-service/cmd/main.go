package main

import (
	"cleaning-app/subscription-service/internal/config"
	"cleaning-app/subscription-service/internal/handler"
	"cleaning-app/subscription-service/internal/repository"
	"cleaning-app/subscription-service/internal/services"
	"cleaning-app/subscription-service/internal/utils"

	"context"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
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

	subService := services.NewSubscriptionService(repo, paymentClient, orderClient)
	// 4. Обработчики
	subHandler := handler.NewSubscriptionHandler(subService, orderClient, paymentClient)

	// 6. Настройка Gin
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.RedirectTrailingSlash = false

	// 7. Middleware
	authMW := utils.AuthMiddleware(cfg.AuthServiceURL)

	api := router.Group("/subscriptions", authMW)
	api.Use(authMW)
	{
		api.POST("", subHandler.Create)
		api.POST("/extend/:id", subHandler.Extend)
		api.GET("/:id", subHandler.GetSubscriptionByIDHTTP)
		api.GET("/my", subHandler.GetMy)
		api.GET("", subHandler.GetAll)
		api.PUT("/:id", subHandler.Update)
		api.DELETE("/:id", subHandler.Cancel)
	}

	utils.StartScheduler(subService)

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
