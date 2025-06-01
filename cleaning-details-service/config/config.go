package config

import (
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"os"
)

type Config struct {
	MongoURI       string
	ServerPort     string
	AuthServiceURL string
	RedisURL       string
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}
	return &Config{
		MongoURI:       os.Getenv("MONGO_URI"),
		ServerPort:     os.Getenv("SERVER_PORT"),
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
		RedisURL:       os.Getenv("REDIS_URL"),
	}, nil
}
