package config

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration
type Config struct {
	APIURL              string
	OutputFile          string
	Timezone            string
	ScheduleTime        string
	ZonomiHosts         []string
	ZonomiAPIKey        string
	ZonomiAPIEncrypted  bool
	ZonomiEncryptionKey string
	MaxRetries          int
	RunOnce             bool
	ZonomiAPIURL        string
}

// New creates a new Config instance from environment variables.
func New() (*Config, error) {

	cfg := &Config{
		APIURL:       getEnv("API_URL", "https://api.ipify.org?format=json"),
		OutputFile:   getEnv("OUTPUT_FILE", "/app/data/ip_log.log"),
		MaxRetries:   getEnvInt("MAX_RETRIES", 3),
		Timezone:     getEnv("TIMEZONE", "Europe/London"),
		ScheduleTime: getEnv("SCHEDULE_TIME", "23:59"),
		ZonomiAPIURL: getEnv("ZONOMI_API_URL", "https://zonomi.com/app/dns/dyndns.jsp"),
		RunOnce:      getEnvBool("RUN_ONCE", false),
	}

	// Load ZONOMI_HOSTS
	hosts, err := loadHosts()
	if err != nil {
		return nil, err
	}

	cfg.ZonomiHosts = hosts

	// Load ZONOMI_API_ENCRYPTED and ZONOMI_ENCRYPTION_KEY
	cfg.ZonomiAPIEncrypted = getEnvBool("ZONOMI_API_ENCRYPTED", false)
	cfg.ZonomiEncryptionKey = getEnv("ZONOMI_ENCRYPTION_KEY", "")

	// Load and possibly decrypt ZONOMI_API_KEY
	apiKey, err := loadAPIKey(cfg.ZonomiAPIEncrypted, cfg.ZonomiEncryptionKey)
	if err != nil {
		return nil, err
	}
	cfg.ZonomiAPIKey = apiKey

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.OutputFile), 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default.
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

// getEnvBool retrieves an environment variable as a boolean or returns a default.
func getEnvBool(key string, defaultValue bool) bool {

	if value, exists := os.LookupEnv(key); exists {
		return value == "true"
	}

	return defaultValue
}

// loadHosts parses the ZONOMI_HOSTS environment variable into a slice of strings.
func loadHosts() ([]string, error) {

	hostsStr := getEnv("ZONOMI_HOSTS", "")
	if hostsStr == "" {
		return nil, fmt.Errorf("ZONOMI_HOSTS is required")
	}

	hosts := strings.Split(hostsStr, ",")
	for i, host := range hosts {
		hosts[i] = strings.TrimSpace(host)
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("ZONOMI_HOSTS is empty after parsing")
	}

	return hosts, nil
}

// decryptAPIKey decrypts an encrypted API key using AES-256-GCM.
func decryptAPIKey(encryptedKey, encryptKey string) (string, error) {

	data, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode ZONOMI_API_KEY: %w", err)
	}

	if len(encryptKey) != 32 {
		return "", fmt.Errorf("ZONOMI_ENCRYPTION_KEY must be 32 bytes, got %d", len(encryptKey))
	}

	block, err := aes.NewCipher([]byte(encryptKey))
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("invalid ciphertext: too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt ZONOMI_API_KEY: %w", err)
	}

	return string(plaintext), nil
}

// loadAPIKey loads and optionally decrypts the ZONOMI_API_KEY.
func loadAPIKey(encrypted bool, encryptKey string) (string, error) {

	apiKey := getEnv("ZONOMI_API_KEY", "")
	if apiKey == "" {
		return "", fmt.Errorf("ZONOMI_API_KEY is required")
	}

	if encrypted {
		if encryptKey == "" {
			return "", fmt.Errorf("ZONOMI_ENCRYPTION_KEY is required when ZONOMI_API_ENCRYPTED is true")
		}
		return decryptAPIKey(apiKey, encryptKey)
	}

	return apiKey, nil
}
