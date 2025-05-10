package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	MongoURI           string
	JWTSecret          string
	ServerPort         string
	AuthServiceURL     string
	RedisURL           string
	NotifiServiceURL   string
	CleaningDetailsURL string
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	return &Config{
		MongoURI:           os.Getenv("MONGO_URI"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		RedisURL:           os.Getenv("REDIS_URL"),
		ServerPort:         os.Getenv("SERVER_PORT"),
		AuthServiceURL:     os.Getenv("AUTH_SERVICE_URL"),
		NotifiServiceURL:   os.Getenv("NOTIFI_SERVICE_URL"),
		CleaningDetailsURL: os.Getenv("CLEANING_DETAILS_URL"),
	}, nil
}
