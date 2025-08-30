package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	// Use a temporary directory for OUTPUT_FILE
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")

	// Set required environment variables
	os.Setenv("OUTPUT_FILE", outputFile)
	os.Setenv("ZONOMI_HOST", "example.com")
	os.Setenv("ZONOMI_API_KEY", "test-api-key")

	// Load config
	cfg, err := Load()
	require.NoError(t, err)

	// Assert default values
	assert.Equal(t, "https://api.ipify.org?format=json", cfg.APIURL)
	assert.Equal(t, outputFile, cfg.OutputFile)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, "Europe/London", cfg.Timezone)
	assert.Equal(t, "23:59", cfg.ScheduleTime)
	assert.Equal(t, "example.com", cfg.ZonomiHost)
	assert.Equal(t, "test-api-key", cfg.ZonomiAPIKey)
	assert.Equal(t, "", cfg.ZonomiEncryptKey)
	assert.False(t, cfg.ZonomiAPIEncrypted)

	// Ensure output directory exists
	_, err = os.Stat(filepath.Dir(cfg.OutputFile))
	assert.NoError(t, err, "Output directory should exist")
}

func TestLoadConfig_EnvironmentOverrides(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	// Use a temporary directory for OUTPUT_FILE
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "log.txt")

	// Set environment variables
	os.Setenv("API_URL", "https://test.api")
	os.Setenv("OUTPUT_FILE", outputFile)
	os.Setenv("MAX_RETRIES", "5")
	os.Setenv("TIMEZONE", "UTC")
	os.Setenv("SCHEDULE_TIME", "12:00")
	os.Setenv("ZONOMI_HOST", "test.host")
	os.Setenv("ZONOMI_API_KEY", "custom-api-key")

	// Load config
	cfg, err := Load()
	require.NoError(t, err)

	// Assert overridden values
	assert.Equal(t, "https://test.api", cfg.APIURL)
	assert.Equal(t, outputFile, cfg.OutputFile)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, "UTC", cfg.Timezone)
	assert.Equal(t, "12:00", cfg.ScheduleTime)
	assert.Equal(t, "test.host", cfg.ZonomiHost)
	assert.Equal(t, "custom-api-key", cfg.ZonomiAPIKey)
	assert.Equal(t, "", cfg.ZonomiEncryptKey)
	assert.False(t, cfg.ZonomiAPIEncrypted)
}

func TestLoadConfig_EncryptedAPIKey(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	// Use a temporary directory for OUTPUT_FILE
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")
	os.Setenv("OUTPUT_FILE", outputFile)

	// Generate a valid 32-byte encryption key
	encryptKey := strings.Repeat("a", 32)

	// Encrypt a test API key
	plainAPIKey := "test-api-key"
	encryptedAPIKey, err := encrypt([]byte(plainAPIKey), []byte(encryptKey))
	require.NoError(t, err)

	// Set environment variables
	os.Setenv("ZONOMI_HOST", "example.com")
	os.Setenv("ZONOMI_API_KEY", encryptedAPIKey)
	os.Setenv("ZONOMI_API_ENCRYPTED", "true")
	os.Setenv("ZONOMI_ENCRYPT_KEY", encryptKey)

	// Load config
	cfg, err := Load()
	require.NoError(t, err)

	// Assert decrypted API key
	assert.Equal(t, plainAPIKey, cfg.ZonomiAPIKey)
	assert.True(t, cfg.ZonomiAPIEncrypted)
	assert.Equal(t, encryptKey, cfg.ZonomiEncryptKey)
}

func TestLoadConfig_MissingHostOrAPIKey(t *testing.T) {
	// Use a temporary directory for OUTPUT_FILE
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")

	tests := []struct {
		name        string
		setHost     bool
		setAPIKey   bool
		expectedErr string
	}{
		{
			name:        "Missing both",
			setHost:     false,
			setAPIKey:   false,
			expectedErr: "ZONOMI_HOST and ZONOMI_API_KEY must be set",
		},
		{
			name:        "Missing API key",
			setHost:     true,
			setAPIKey:   false,
			expectedErr: "ZONOMI_HOST and ZONOMI_API_KEY must be set",
		},
		{
			name:        "Missing host",
			setHost:     false,
			setAPIKey:   true,
			expectedErr: "ZONOMI_HOST and ZONOMI_API_KEY must be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			os.Clearenv()

			// Set OUTPUT_FILE
			os.Setenv("OUTPUT_FILE", outputFile)

			// Set variables based on test case
			if tt.setHost {
				os.Setenv("ZONOMI_HOST", "example.com")
			}
			if tt.setAPIKey {
				os.Setenv("ZONOMI_API_KEY", "test-api-key")
			}

			// Load config
			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestLoadConfig_InvalidTimezone(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	// Use a temporary directory for OUTPUT_FILE
	tempDir := t.TempDir()
	os.Setenv("OUTPUT_FILE", filepath.Join(tempDir, "ip_log.txt"))

	// Set invalid timezone
	os.Setenv("TIMEZONE", "Invalid/Timezone")
	os.Setenv("ZONOMI_HOST", "example.com")
	os.Setenv("ZONOMI_API_KEY", "test-api-key")

	// Load config
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timezone")
}

func TestLoadConfig_EncryptedAPIKeyMissingEncryptKey(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	// Use a temporary directory for OUTPUT_FILE
	tempDir := t.TempDir()
	os.Setenv("OUTPUT_FILE", filepath.Join(tempDir, "ip_log.txt"))

	// Set encrypted API key without encryption key
	os.Setenv("ZONOMI_HOST", "example.com")
	os.Setenv("ZONOMI_API_KEY", "some-encrypted-key")
	os.Setenv("ZONOMI_API_ENCRYPTED", "true")

	// Load config
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ZONOMI_ENCRYPT_KEY is required")
}

func TestLoadConfig_InvalidEncryptedAPIKey(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	// Use a temporary directory for OUTPUT_FILE
	tempDir := t.TempDir()
	os.Setenv("OUTPUT_FILE", filepath.Join(tempDir, "ip_log.txt"))

	// Set invalid encrypted API key
	os.Setenv("ZONOMI_HOST", "example.com")
	os.Setenv("ZONOMI_API_KEY", "invalid-base64")
	os.Setenv("ZONOMI_API_ENCRYPTED", "true")
	os.Setenv("ZONOMI_ENCRYPT_KEY", strings.Repeat("a", 32))

	// Load config
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt ZONOMI_API_KEY")
}

// encrypt is a helper function for tests, mirroring the encryption logic in README.md
func encrypt(plaintext, key []byte) (string, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
