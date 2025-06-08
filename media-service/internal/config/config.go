package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	MongoURI               string
	MinioEndpoint          string
	MinioAccessKey         string
	MinioSecretKey         string
	MinioBucket            string
	JWTSecret              string
	NotificationServiceURL string
	AuthServiceURL         string
	OrderServiceURL        string
	MinioPublicURL         string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()
	return &Config{
		MongoURI:               os.Getenv("MONGO_URI"),
		MinioEndpoint:          os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey:         os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:         os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:            os.Getenv("MINIO_BUCKET"),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		NotificationServiceURL: os.Getenv("NOTIFICATION_SERVICE_URL"),
		AuthServiceURL:         os.Getenv("AUTH_SERVICE_URL"),
		OrderServiceURL:        os.Getenv("ORDER_SERVICE_URL"),
		MinioPublicURL:         os.Getenv("MINIO_PUBLIC_URL"),
	}, nil
}
