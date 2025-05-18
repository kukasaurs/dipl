package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	MongoURI               string
	MongoDB                string
	NotificationServiceURL string
	Port                   string
	JWTSecret              string
	AuthServiceURL         string
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	return &Config{
		MongoURI:               os.Getenv("MONGO_URI"),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		MongoDB:                os.Getenv("MONGO_DB"),
		NotificationServiceURL: os.Getenv("NOTIFICATION_SERVICE_URL"),
		Port:                   os.Getenv("PORT"),
		AuthServiceURL:         os.Getenv("AUTH_SERVICE_URL"),
	}, nil
}
