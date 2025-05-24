package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"cleaning-app/api-gateway/internal/proxy"
	"cleaning-app/api-gateway/internal/utils"
	"cleaning-app/api-gateway/setup"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	authURL := os.Getenv("AUTH_SERVICE_URL")

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Public routes
	r.Any("/api/auth/*proxyPath", proxy.CreateProxy(
		"http://auth-service:8000",
		"/api/auth",
		"/auth",
	))

	r.Any("/api/payments", proxy.CreateProxy(
		"http://payment-service:8005",
		"/api/payments",
		"/payments",
	))
	// И всё остальное ниже /api/payments/...
	r.Any("/api/payments/*proxyPath", proxy.CreateProxy(
		"http://payment-service:8005",
		"/api/payments",
		"/payments",
	))

	log.Printf("AUTH_SERVICE_URL: %s", authURL)

	// Secured routes
	secured := r.Group("/api")
	secured.Use(utils.AuthMiddleware(authURL))
	setup.ConfigureServiceProxies(secured)

	// Admin routes
	admin := secured.Group("/admin")
	admin.Use(utils.RequireRoles("admin"))
	setup.ConfigureAdminProxies(admin)

	// Server setup
	srv := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: r,
	}

	log.Println("API Gateway listening on :8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to run API Gateway: %v", err)
	}
}
