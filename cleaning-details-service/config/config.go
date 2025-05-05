package config

import (
	"github.com/caarlos0/env/v10"
	_ "github.com/joho/godotenv/autoload"

	"cleaning-app/cleaning-details-service/utils/mongodb"
)

// Config holds all application configuration
type Config struct {
	MongoDB     mongodb.Config
	Server      ServerConfig
	AuthService AuthServiceConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string `env:"SERVER_PORT" envDefault:"8083"`
}

// AuthServiceConfig holds auth service configuration
type AuthServiceConfig struct {
	URL string `env:"AUTH_SERVICE_URL" envDefault:"http://auth-service:8081"`
}

// NewConfig creates a new Config
func NewConfig() (*Config, error) {
	cfg := new(Config)
	err := env.Parse(cfg)

	return cfg, err
}
