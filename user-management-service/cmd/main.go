package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"cleaning-app/user-management-service/internal/config"
	"cleaning-app/user-management-service/internal/handler"
	"cleaning-app/user-management-service/internal/repository"
	"cleaning-app/user-management-service/internal/services"
	"cleaning-app/user-management-service/internal/utils"
)

func main() {
	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// MongoDB
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Mongo connect:", err)
	}
	db := client.Database("cleaning_service")
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Disconnecting MongoDB...")
		return client.Disconnect(ctx)
	})

	repo := repository.NewUserRepository(db)
	svc := services.NewUserService(repo)
	h := handler.NewUserHandler(svc)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.RedirectTrailingSlash = false

	authMW := utils.AuthMiddleware(cfg.AuthServiceURL)

	users := router.Group("/users")
	users.Use(authMW)
	{
		// доступно всем аутентифицированным
		users.GET("/me", h.GetMe)
		users.GET("/:id", h.GetUserByID)

		// менеджер и админ могут смотреть список и создавать
		mgrAdmin := users.Group("")
		mgrAdmin.Use(utils.RequireRoles("admin", "manager"))
		{
			mgrAdmin.GET("", h.GetAllUsers)
			mgrAdmin.POST("", h.CreateUser)
		}

		// только админ
		adminOnly := users.Group("")
		adminOnly.Use(utils.RequireRoles("admin"))
		{
			adminOnly.PUT("/:id/role", h.ChangeUserRole)
			adminOnly.PUT("/:id/block", h.BlockUser)
			adminOnly.PUT("/:id/unblock", h.UnblockUser)
		}
	}

	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}
	go func() {
		log.Println("User service listening on", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] HTTP server shutting down...")
		return srv.Shutdown(ctx)
	})

	select {}
}
