package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr          string
	TableName           string
	DurableFunctionName string
	AllowedOrigin       string
	CartTTL             time.Duration
	PixTTL              time.Duration
	CardDelay           time.Duration
	CardDelayLong       time.Duration
	SessionTTL          time.Duration
}

func FromEnv() Config {
	return Config{
		ListenAddr:          env("LISTEN_ADDR", "127.0.0.1:8080"),
		TableName:           os.Getenv("TABLE_NAME"),
		DurableFunctionName: os.Getenv("DURABLE_FUNCTION_NAME"),
		AllowedOrigin:       env("ALLOWED_ORIGIN", "*"),
		CartTTL:             envDuration("CART_TTL_SECONDS", 15*time.Minute),
		PixTTL:              envDuration("PIX_TTL_SECONDS", 15*time.Minute),
		CardDelay:           envDuration("CARD_DELAY_SECONDS", 8*time.Second),
		CardDelayLong:       envDuration("CARD_DELAY_LONG_SECONDS", 20*time.Second),
		SessionTTL:          envDuration("SESSION_TTL_SECONDS", 7*24*time.Hour),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}
	return time.Duration(n) * time.Second
}
