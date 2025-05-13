package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	MongoURI         string
	JWTSecret        string
	RedisURL         string
	GoogleClientID   string
	SMTPHost         string
	SMTPPort         string
	SMTPUser         string
	SMTPPass         string
	NotifiServiceURL string
}


func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	return &Config{
		MongoURI:         os.Getenv("MONGO_URI"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		RedisURL:         os.Getenv("REDIS_URL"),
		GoogleClientID:   os.Getenv("GOOGLE_CLIENT_ID"),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPPort:         os.Getenv("SMTP_PORT"),
		SMTPUser:         os.Getenv("SMTP_USER"),
		SMTPPass:         os.Getenv("SMTP_PASS"),
		NotifiServiceURL: os.Getenv("NOTIFI_SERVICE_URL"),
	}, nil
}
