package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

// Config parameters for MongoDB connection
type Config struct {
	Host     string `env:"MONGO_HOST" envDefault:"localhost"`
	Port     int    `env:"MONGO_PORT" envDefault:"27017"`
	User     string `env:"MONGO_USER"`
	Password string `env:"MONGO_PASSWORD"`
	DBName   string `env:"MONGO_DBNAME" envDefault:"cleaning_service"`
}

// NewMongoDBConnection creates a new connection to MongoDB
func NewMongoDBConnection(cfg Config) (*mongo.Client, error) {
	var uri string
	if cfg.User != "" && cfg.Password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d", cfg.User, cfg.Password, cfg.Host, cfg.Port)
	} else {
		uri = fmt.Sprintf("mongodb://%s:%d", cfg.Host, cfg.Port)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Ping the database to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	return client, nil
}
