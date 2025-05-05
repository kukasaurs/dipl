package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	MongoURI       string
	RedisURL       string
	AuthServiceURL string
}

// LoadConfig подгружает переменные окружения из .env
func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}
	return &Config{
		MongoURI:       os.Getenv("MONGO_URI"),
		RedisURL:       os.Getenv("REDIS_URL"),
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
	}, nil
}
