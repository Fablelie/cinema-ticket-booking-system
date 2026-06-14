package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                string
	MongoURI            string
	RedisURI            string
	RabbitMQURI         string
	DefaultShowtimeID   string
	GoogleClientID      string
	AdminEmails         string
	SeatLockTTL         time.Duration
}

func LoadConfig() *Config {
	// Parse SEAT_LOCK_TTL_SECONDS (default 300 seconds = 5 minutes)
	seatLockTTLSeconds, err := strconv.Atoi(getEnv("SEAT_LOCK_TTL_SECONDS", "300"))
	if err != nil || seatLockTTLSeconds <= 0 {
		seatLockTTLSeconds = 300 // fallback to 5 minutes
	}

	return &Config{
		Port:                getEnv("PORT", "8080"),
		MongoURI:            getEnv("MONGO_URI", "mongodb://localhost:27017"),
		RedisURI:            getEnv("REDIS_URI", "localhost:6379"),
		RabbitMQURI:         getEnv("RABBITMQ_URI", "amqp://guest:guest@localhost:5672/"),
		DefaultShowtimeID:   getEnv("VITE_DEFAULT_SHOWTIME_ID", "64b1f0000000000000000001"),
		GoogleClientID:      getEnv("VITE_GOOGLE_CLIENT_ID", ""),
		AdminEmails:         getEnv("VITE_ADMIN_EMAILS", ""),
		SeatLockTTL:         time.Duration(seatLockTTLSeconds) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
