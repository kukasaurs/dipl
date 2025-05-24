package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	MongoURI               string
	MongoDB                string
	NotificationServiceURL string
	JWTSecret              string
	AuthServiceURL         string
	ServerPort             string
	UserServiceURL         string
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
		AuthServiceURL:         os.Getenv("AUTH_SERVICE_URL"),
		ServerPort:             os.Getenv("SERVER_PORT"),
		UserServiceURL:         os.Getenv("USER_SERVICE_URL"),
	}, nil
}
