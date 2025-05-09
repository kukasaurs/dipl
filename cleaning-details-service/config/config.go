package config

import (
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"os"
)

// Config holds all application configuration
// config/config.go
type Config struct {
	MongoURI       string
	ServerPort     string
	AuthServiceURL string
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}
	return &Config{
		MongoURI:       os.Getenv("MONGO_URI"),
		ServerPort:     os.Getenv("SERVER_PORT"),
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
	}, nil
}
