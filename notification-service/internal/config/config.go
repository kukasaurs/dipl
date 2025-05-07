package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	MongoURI            string
	RedisURL            string
	ServerPort          string
	AuthServiceURL      string
	SMTPHost            string
	SMTPPort            string
	SMTPUser            string
	SMTPPass            string
	FirebaseProjectID   string
	FirebaseCredentials string
	FirebaseKey         string
}

// LoadConfig подгружает переменные окружения из .env
func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}
	return &Config{
		MongoURI:            os.Getenv("MONGO_URI"),
		RedisURL:            os.Getenv("REDIS_URL"),
		ServerPort:          os.Getenv("SERVER_PORT"),
		AuthServiceURL:      os.Getenv("AUTH_SERVICE_URL"),
		SMTPHost:            os.Getenv("SMTP_HOST"),
		SMTPPort:            os.Getenv("SMTP_PORT"),
		SMTPUser:            os.Getenv("SMTP_USER"),
		SMTPPass:            os.Getenv("SMTP_PASS"),
		FirebaseProjectID:   os.Getenv("FIREBASE_PROJECT_ID"),
		FirebaseCredentials: os.Getenv("FIREBASE_CREDENTIALS_FILE"),
		FirebaseKey:         os.Getenv("FCM_SERVER_KEY"),
	}, nil
}
