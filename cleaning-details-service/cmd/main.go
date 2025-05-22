package main

import (
	"cleaning-app/cleaning-details-service/config"
	handlers "cleaning-app/cleaning-details-service/internal/handler"
	"cleaning-app/cleaning-details-service/internal/repository"
	services "cleaning-app/cleaning-details-service/internal/service"
	"cleaning-app/cleaning-details-service/utils"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
		log.Fatalf("Error parsing configs: %v", err)
	}

	// 2. Redis
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal("Invalid Redis URL:", err)
	}

	redisClient := redis.NewClient(opts)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	} else {
		log.Println("Connected to Redis successfully")
	}

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing Redis connection...")
		return redisClient.Close()
	})

	// 3. MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	db := mongoClient.Database("cleaning_service")

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// 4. Инициализация компонентов
	serviceRepo := repository.NewCleaningServiceRepository(db)
	serviceSrv := services.NewCleaningService(serviceRepo, redisClient)
	serviceHandler := handlers.NewCleaningServiceHandler(serviceSrv)

	// 5. Инициализация Gin
	router := gin.Default()
	router.RedirectTrailingSlash = false
	// 6. Публичные маршруты
	public := router.Group("/api/services")
	{
		public.GET("/active", serviceHandler.GetActiveServices)
		public.POST("/by-ids", serviceHandler.GetServicesByIDs)
	}

	// 7. Админ-маршруты с авторизацией
	admin := router.Group("/admin")
	admin.Use(utils.AuthMiddleware(cfg.AuthServiceURL))
	admin.Use(utils.RequireRoles("admin"))
	{
		admin.GET("/services", serviceHandler.GetAllServices)
		admin.POST("/services", serviceHandler.CreateService)
		admin.PUT("/services", serviceHandler.UpdateService)
		admin.DELETE("/services/:id", serviceHandler.DeleteService)
		admin.PATCH("services/:id/status", serviceHandler.ToggleServiceStatus)
	}

	// 8. Запуск сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Printf("Server started on port %s", cfg.ServerPort)
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
