package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	UAZAPI UAZAPIConfig
	Chatwoot ChatwootConfig
	Sync SyncConfig
}

type UAZAPIConfig struct {
	BaseURL string
	Token   string
}

type ChatwootConfig struct {
	DB       DBConfig
	API      APIConfig
	AccountID int
	InboxID   int
	InboxName string
}

type DBConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

type APIConfig struct {
	BaseURL string
	Token   string
}

type SyncConfig struct {
	BatchSize      int
	LimitChats     int
	LimitMessages  int
}

func Load() (*Config, error) {
	// Load .env file if exists
	_ = godotenv.Load()

	cfg := &Config{
		UAZAPI: UAZAPIConfig{
			BaseURL: getEnv("UAZAPI_BASE_URL", "https://free.uazapi.com"),
			Token:   getEnv("UAZAPI_TOKEN", ""),
		},
		Chatwoot: ChatwootConfig{
			DB: DBConfig{
				Host:     getEnv("CHATWOOT_DB_HOST", "localhost"),
				Port:     getEnvAsInt("CHATWOOT_DB_PORT", 5432),
				Name:     getEnv("CHATWOOT_DB_NAME", "chatwoot"),
				User:     getEnv("CHATWOOT_DB_USER", "chatwoot"),
				Password: getEnv("CHATWOOT_DB_PASSWORD", ""),
				SSLMode:  getEnv("CHATWOOT_DB_SSLMODE", "disable"),
			},
			API: APIConfig{
				BaseURL: getEnv("CHATWOOT_BASE_URL", ""),
				Token:   getEnv("CHATWOOT_API_TOKEN", ""),
			},
			AccountID: getEnvAsInt("CHATWOOT_ACCOUNT_ID", 1),
			InboxID:   getEnvAsInt("CHATWOOT_INBOX_ID", 1),
			InboxName: getEnv("CHATWOOT_INBOX_NAME", "WhatsApp"),
		},
		Sync: SyncConfig{
			BatchSize:     getEnvAsInt("SYNC_BATCH_SIZE", 1000),
			LimitChats:    getEnvAsInt("SYNC_LIMIT_CHATS", 100000),
			LimitMessages: getEnvAsInt("SYNC_LIMIT_MESSAGES", 10000),
		},
	}

	// Validate required fields
	if cfg.UAZAPI.Token == "" {
		return nil, fmt.Errorf("UAZAPI_TOKEN is required")
	}
	if cfg.Chatwoot.DB.Password == "" {
		return nil, fmt.Errorf("CHATWOOT_DB_PASSWORD is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

