package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"cleaning-app/cleaning-details-service/config"
	"cleaning-app/cleaning-details-service/internal/handler"
	"cleaning-app/cleaning-details-service/internal/repository"
	"cleaning-app/cleaning-details-service/internal/service"
	"cleaning-app/cleaning-details-service/utils"
)

func main() { //comment for nurda

	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error parsing configs: %v", err)
	}

	// Redis
	redisClient := utils.NewRedisClient(cfg.RedisURL)

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	} else {
		log.Println("Connected to Redis successfully")
	}

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing Redis connection...")
		return utils.CloseRedis(ctx, redisClient)
	})

	// Connect to MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	db := mongoClient.Database("cleaning_service")

	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Closing MongoDB connection...")
		return mongoClient.Disconnect(ctx)
	})

	// Initialize components
	serviceRepo := repository.NewCleaningServiceRepository(db)
	serviceSrv := services.NewCleaningService(serviceRepo, redisClient)
	serviceHandler := handlers.NewCleaningServiceHandler(serviceSrv)
	authClient := utils.NewAuthClient(cfg.AuthServiceURL)

	// Setup router
	router := mux.NewRouter()
	router.Use(utils.LoggingMiddleware)

	// Public endpoints
	publicRouter := router.PathPrefix("/api/services").Subrouter()
	publicRouter.HandleFunc("/active", serviceHandler.GetActiveServices).Methods(http.MethodGet)
	publicRouter.HandleFunc("/by-ids", serviceHandler.GetServicesByIDs).Methods(http.MethodPost)

	// Admin endpoints with authentication
	adminRouter := router.PathPrefix("/api/admin/services").Subrouter()
	adminRouter.Use(utils.JWTWithAuth(authClient, "admin"))

	adminRouter.HandleFunc("", serviceHandler.GetAllServices).Methods(http.MethodGet)
	adminRouter.HandleFunc("", serviceHandler.CreateService).Methods(http.MethodPost)
	adminRouter.HandleFunc("", serviceHandler.UpdateService).Methods(http.MethodPut)
	adminRouter.HandleFunc("/{id}", serviceHandler.DeleteService).Methods(http.MethodDelete)
	adminRouter.HandleFunc("/{id}/status", serviceHandler.ToggleServiceStatus).Methods(http.MethodPatch)

	// Start server
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	log.Printf("Server started on port %s", cfg.ServerPort)
	log.Fatal(server.ListenAndServe())
}
