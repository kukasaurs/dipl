// 📁 notification-service/main.go
package main

import (
	"cleaning-app/notification-service/internal/handler"
	"cleaning-app/notification-service/internal/repository"
	service "cleaning-app/notification-service/internal/services"
	"cleaning-app/notification-service/internal/utils"
	"context"
	"github.com/redis/go-redis/v9"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	r := gin.Default()
	ctx := context.Background()

	// Mongo setup
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	db := mongoClient.Database("notifications")
	notifRepo := repository.NewMongoNotificationRepo(db)

	// Redis setup
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	notifService := service.NewNotificationService(notifRepo)
	go utils.SubscribeToRedis(ctx, redisClient, notifService)

	h := http.NewHandler(notifService)
	r.GET("/notifications", h.GetNotifications)
	r.PUT("/notifications/:id/read", h.MarkAsRead)

	s := &http.Server{
		Addr:         ":8002",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Println("Notification Service running on :8002")
	s.ListenAndServe()
}
