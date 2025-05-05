package main

import (
	"cleaning-app/cleaning-details-service/config"
	"cleaning-app/cleaning-details-service/internal/handler"
	"cleaning-app/cleaning-details-service/internal/repository"
	"cleaning-app/cleaning-details-service/internal/service"
	"cleaning-app/cleaning-details-service/utils/auth"
	"cleaning-app/cleaning-details-service/utils/middleware"
	"cleaning-app/cleaning-details-service/utils/mongodb"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func main() {
	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Error parsing configs: %v", err)
	}

	// Connect to MongoDB
	client, err := mongodb.NewMongoDBConnection(cfg.MongoDB)
	if err != nil {
		log.Fatalf("Error connecting to MongoDB: %v", err)
	}
	defer client.Disconnect(nil)

	// Initialize components
	serviceRepo := repository.NewCleaningServiceRepository(client.Database(cfg.MongoDB.DBName))
	serviceSrv := services.NewCleaningService(serviceRepo)
	serviceHandler := handlers.NewCleaningServiceHandler(serviceSrv)
	authClient := auth.NewAuthClient(cfg.AuthService.URL)

	// Setup router
	router := mux.NewRouter()
	router.Use(middleware.LoggingMiddleware)

	// Public endpoints
	publicRouter := router.PathPrefix("/api/services").Subrouter()
	publicRouter.HandleFunc("/active", serviceHandler.GetActiveServices).Methods(http.MethodGet)

	// Admin endpoints with authentication
	adminRouter := router.PathPrefix("/api/admin/services").Subrouter()
	adminRouter.Use(auth.JWTWithAuth(authClient, "ADMIN"))

	adminRouter.HandleFunc("", serviceHandler.GetAllServices).Methods(http.MethodGet)
	adminRouter.HandleFunc("", serviceHandler.CreateService).Methods(http.MethodPost)
	adminRouter.HandleFunc("", serviceHandler.UpdateService).Methods(http.MethodPut)
	adminRouter.HandleFunc("/{id}", serviceHandler.DeleteService).Methods(http.MethodDelete)
	adminRouter.HandleFunc("/{id}/status", serviceHandler.ToggleServiceStatus).Methods(http.MethodPatch)

	// Start server
	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	log.Printf("Server started on port %s", cfg.Server.Port)
	log.Fatal(server.ListenAndServe())
}
