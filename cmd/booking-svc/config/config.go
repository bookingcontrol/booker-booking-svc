package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port            int
	MetricsPort    int
	Env             string
	PostgresHost    string
	PostgresPort    int
	PostgresDB      string
	PostgresUser    string
	PostgresPassword string
	RedisAddr       string
	RedisPassword   string
	KafkaBrokers    string
	GRPCVenueAddr   string
	JaegerEndpoint  string
	HoldTTLMinutes  int
}

func Load() *Config {
	return &Config{
		Port:            getEnvInt("PORT", 50052),
		MetricsPort:     getEnvInt("METRICS_PORT", 9092),
		Env:             getEnv("ENV", "development"),
		PostgresHost:    getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:    getEnvInt("POSTGRES_PORT", 5432),
		PostgresDB:      getEnv("POSTGRES_DB", "booking"),
		PostgresUser:    getEnv("POSTGRES_USER", "booking_user"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "booking_pass"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		KafkaBrokers:    getEnv("KAFKA_BROKERS", "localhost:9092"),
		GRPCVenueAddr:   getEnv("GRPC_VENUE_ADDR", "localhost:50051"),
		JaegerEndpoint:  getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
		HoldTTLMinutes:  getEnvInt("HOLD_TTL_MINUTES", 10),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}



