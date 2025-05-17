package main

import (
	"cleaning-app/user-management-service/internal/config"
	"cleaning-app/user-management-service/internal/handler"
	"cleaning-app/user-management-service/internal/repository"
	"cleaning-app/user-management-service/internal/services"
	"cleaning-app/user-management-service/internal/utils"
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

	// 2. Инициализация MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	db := mongoClient.Database("cleaning_service")

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// 3. Инициализация слоёв: репозиторий → сервис → обработчик

	userRepo := repository.NewUserRepository(db)
	userService := services.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	// 4. Настройка Gin и middleware
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://host.docker.internal:8006"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	authMW := utils.AuthMiddleware(cfg.AuthServiceURL)

	api := router.Group("/api/users")
	api.Use(authMW)
	{
		api.GET("/me", userHandler.GetMe)

		managerGroup := api.Group("/")
		managerGroup.Use(utils.RequireRoles("admin", "manager"))
		{
			managerGroup.POST("", userHandler.CreateUser)
			managerGroup.GET("", userHandler.GetAllUsers)
		}

		adminGroup := api.Group("/")
		adminGroup.Use(utils.RequireRoles("admin"))
		{
			adminGroup.PUT("/:id/role", userHandler.ChangeUserRole)
			adminGroup.PUT("/:id/block", userHandler.BlockUser)
			adminGroup.PUT("/:id/unblock", userHandler.UnblockUser)
		}
	}

	// 5. Запуск сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Println("User Management Service running on port", cfg.ServerPort)
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
