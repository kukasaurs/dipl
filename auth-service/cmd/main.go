package main

import (
	"cleaning-app/auth-service/internal/config"
	handlers "cleaning-app/auth-service/internal/handler"
	repositories "cleaning-app/auth-service/internal/repository"
	"cleaning-app/auth-service/internal/services"
	"cleaning-app/auth-service/internal/utils"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	_ "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	_ "os"
	_ "time"
)

func main() {
	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

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

	// Зарегистрировать Mongo для Graceful Shutdown
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// 3. Инициализация Redis

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal("Invalid Redis URL:", err)
	}

	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to ping Redis:", err)
	}

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing Redis connection...")
		return rdb.Close()
	})

	// 4. Email сервис (SMTP)
	emailService := services.NewSMTPMailer(
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPUser,
		cfg.SMTPPass,
	)

	// 5. Инициализация репозиториев и сервисов
	userRepo := repositories.NewUserRepository(db)
	jwtUtil := utils.NewJWTUtil(cfg.JWTSecret)
	googleAuth := services.NewGoogleAuthService(cfg.GoogleClientID)

	authService := services.NewAuthService(userRepo, jwtUtil, googleAuth, emailService, utils.WrapRedisClient(rdb), cfg)

	authHandler := handlers.NewAuthHandler(authService)

	// 6. Настройка Gin сервера
	router := gin.Default()

	auth := router.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/google-login", authHandler.GoogleLogin)
		auth.POST("/resend-password", authHandler.ResendPassword)

		auth.GET("/validate", authHandler.Validate)
		auth.GET("/managers", authHandler.GetManagers)
		auth.POST("/logout", authHandler.Logout)

		protected := auth.Group("/")
		protected.Use(utils.AuthMiddleware(jwtUtil, utils.WrapRedisClient(rdb)))
		{
			protected.GET("/profile", authHandler.GetProfile)
			protected.PUT("/profile", authHandler.UpdateProfile)
			protected.PUT("/change-password", authHandler.ChangePassword)
			protected.PUT("/set-initial-password", authHandler.SetInitialPassword)
		}
	}

	server := &http.Server{
		Addr:    "0.0.0.0:8000",
		Handler: router,
	}

	// 7. Запускаем сервер
	go func() {
		log.Println("Auth service running on :8000")
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
