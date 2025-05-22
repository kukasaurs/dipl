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

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Mongo connection failed:", err)
	}
	db := mongoClient.Database("cleaning_service")

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	repo := repository.NewSupportRepository(db)
	notifier := utils.NewNotificationClient(cfg.NotificationServiceURL)

	supportService := services.NewSupportService(repo, notifier)
	supportHandler := handler.NewSupportHandler(supportService)

	router := gin.Default()
	authMW := utils.AuthMiddleware(cfg.AuthServiceURL)

	api := router.Group("/support", authMW)
	{
		api.POST("/tickets", supportHandler.CreateTicket)
		api.GET("/tickets/my", supportHandler.GetMyTickets)
		api.GET("/tickets", supportHandler.GetAllTickets)
		api.PUT("/tickets/:id/status", supportHandler.UpdateTicketStatus)
		api.POST("/tickets/:id/messages", supportHandler.SendMessage)
		api.GET("/tickets/:id/messages", supportHandler.GetMessages)
	}

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
