package main

import (
	"cleaning-app/support-service/internal/config"
	"cleaning-app/support-service/internal/handler"
	"cleaning-app/support-service/internal/repository"
	"cleaning-app/support-service/internal/services"
	"cleaning-app/support-service/internal/utils"
	"context"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
)

func main() {
	// 1. Настройка контекста и shutdown-менеджера
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
		log.Fatal("Mongo connection failed:", err)
	}
	db := mongoClient.Database("cleaning_service")

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// 4. Инициализация репозитория и сервиса (без логики уведомлений)
	repo := repository.NewSupportRepository(db)
	supportService := services.NewSupportService(repo)
	supportHandler := handler.NewSupportHandler(supportService, cfg.UserServiceURL)

	// 5. Настройка Gin
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.RedirectTrailingSlash = false

	// 6. Middleware аутентификации
	authMW := utils.AuthMiddleware(cfg.AuthServiceURL)

	api := router.Group("/support", authMW)
	{
		api.POST("/tickets", supportHandler.CreateTicket)
		api.GET("/tickets", supportHandler.GetTickets)
		api.PUT("/tickets/:id/status", supportHandler.UpdateTicketStatus)
		api.POST("/tickets/:id/messages", supportHandler.SendMessage)
		api.GET("/tickets/:id/messages", supportHandler.GetMessages)
	}

	// 7. Запуск HTTP-сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Println("Support service running on", cfg.ServerPort)
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
