package config

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds application configuration.
type Config struct {
	APIURL             string
	ZonomiAPIURL       string
	OutputFile         string
	MaxRetries         int
	Timezone           string
	ScheduleTime       string
	ZonomiHost         string
	ZonomiAPIKey       string
	ZonomiEncryptKey   string
	ZonomiAPIEncrypted bool
}

// Load retrieves configuration from environment variables with defaults.
func Load() (Config, error) {
	cfg := Config{
		APIURL:             getEnv("API_URL", "https://api.ipify.org?format=json"),
		ZonomiAPIURL:       getEnv("ZONOMI_API_URL", "https://zonomi.com/app/dns/dyndns.jsp"),
		OutputFile:         getEnv("OUTPUT_FILE", "/app/data/ip_log.txt"),
		MaxRetries:         getEnvInt("MAX_RETRIES", 3),
		Timezone:           getEnv("TIMEZONE", "Europe/London"),
		ScheduleTime:       getEnv("SCHEDULE_TIME", "23:59"),
		ZonomiHost:         getEnv("ZONOMI_HOST", ""),
		ZonomiAPIKey:       getEnv("ZONOMI_API_KEY", ""),
		ZonomiEncryptKey:   getEnv("ZONOMI_ENCRYPT_KEY", ""),
		ZonomiAPIEncrypted: getEnv("ZONOMI_API_ENCRYPTED", "false") == "true",
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.OutputFile), 0755); err != nil {
		return Config{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Validate timezone
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return Config{}, fmt.Errorf("invalid timezone: %w", err)
	}

	// Handle encrypted API key
	if cfg.ZonomiAPIEncrypted {
		if cfg.ZonomiEncryptKey == "" {
			return Config{}, fmt.Errorf("ZONOMI_ENCRYPT_KEY is required when ZONOMI_API_ENCRYPTED is true")
		}
		decrypted, err := decrypt(cfg.ZonomiAPIKey, []byte(cfg.ZonomiEncryptKey))
		if err != nil {
			return Config{}, fmt.Errorf("failed to decrypt ZONOMI_API_KEY: %w", err)
		}
		cfg.ZonomiAPIKey = decrypted
	}

	// Validate Zonomi host and API key
	if cfg.ZonomiHost == "" || cfg.ZonomiAPIKey == "" {
		return Config{}, fmt.Errorf("ZONOMI_HOST and ZONOMI_API_KEY must be set")
	}

	return cfg, nil
}

// decrypt decrypts the ciphertext using AES-GCM with the given key.
func decrypt(ciphertext string, key []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm failed: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("gcm open failed: %w", err)
	}

	return string(plaintext), nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an environment variable as an integer or returns a default.
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err == nil {
			return n
		}
	}
	return defaultValue
}
