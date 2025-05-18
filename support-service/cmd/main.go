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

	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

	// Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Mongo connection failed:", err)
	}

	// Регистрация завершения работы MongoDB
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	db := mongoClient.Database(cfg.MongoDB)

	// Репозиторий, уведомления и сервис сообщений
	repo := repository.NewChatRepository(db)
	notifier := services.NewNotifierService(cfg.NotificationServiceURL)
	chatService := services.NewChatService(repo, notifier)
	chatHandler := handler.NewChatHandler(chatService)

	// Роутер и эндпоинты
	router := gin.Default()

	authMiddleware := utils.AuthMiddleware(cfg.AuthServiceURL)

	api := router.Group("/api/support")
	{
		api.POST("/send", authMiddleware, chatHandler.SendMessage)
		api.GET("/messages", authMiddleware, chatHandler.GetMessages)
	}

	// HTTP-сервер
	server := &http.Server{
		Addr:    ":8007",
		Handler: router,
	}

	// Запуск HTTP-сервера
	go func() {
		log.Println("Support service running on :8007")
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
