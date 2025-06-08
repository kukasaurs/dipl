package main

import (
	"cleaning-app/media-service/internal/config"
	"cleaning-app/media-service/internal/handler"
	"cleaning-app/media-service/internal/repository"
	service "cleaning-app/media-service/internal/services"

	"cleaning-app/media-service/internal/utils"
	"context"
	"errors"
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
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	db := mongoClient.Database("cleaning_service")

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	minioClient, err := utils.NewMinioClient(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket)
	if err != nil {
		log.Fatalf("minio init: %v", err)
	}

	repo := repository.NewMediaRepository(db)
	svc := service.NewMediaService(repo, minioClient, cfg.MinioBucket)
	orderClient := utils.NewOrderClient(cfg.OrderServiceURL)
	handler := handler.NewMediaHandler(svc, orderClient)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.RedirectTrailingSlash = false

	router.Use(utils.AuthMiddleware(cfg.AuthServiceURL))
	authMW := utils.AuthMiddleware(cfg.AuthServiceURL)

	media := router.Group("/media")
	media.Use(authMW)
	{
		media.POST("/report/:orderId", handler.UploadReport)
		media.GET("/reports/:orderId", handler.GetReports)
		media.POST("/avatar", handler.UploadAvatar)
		media.GET("/avatars", handler.GetAvatars)
	}

	server := &http.Server{
		Addr:    "0.0.0.0:8007",
		Handler: router,
	}

	go func() {
		log.Println("Media Service running on port 8007")
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
