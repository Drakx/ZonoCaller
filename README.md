# ZonoCaller

A Go application that schedules a daily task to fetch the public IP, check for changes, and update Zonomi DNS if necessary.
Runs in Docker with health checks and supports encrypted API keys.

> **Note** This does not implement all APIs supported by [Zonomi](https://zonomi.com)

## Features
- Scheduled daily IP fetch at configurable time (default: 23:59 CEST).
- IP change detection with persistent logging.
- Zonomi DNS update on IP change.
- Health check endpoint at `/health`.
- Run-once mode for testing.
- Encrypted Zonomi API key support.
- Unit tests for core functionality.

## Configuration
All configurations are via environment variables:
- `API_URL`: IP fetch API (default: https://api.ipify.org?format=json)
- `OUTPUT_FILE`: IP log file path (default: /app/data/ip_log.txt)
- `MAX_RETRIES`: Max retries for API calls (default: 3)
- `TIMEZONE`: Time zone (default: Europe/Paris)
- `SCHEDULE_TIME`: Schedule time (format: HH:MM, default: 23:59)
- `ZONOMI_HOST`: Zonomi host (default: myhost)
- `ZONOMI_API_KEY`: Zonomi API key (required)
- `ZONOMI_API_ENCRYPTED`: Set to "true" if API key is encrypted (default: false)
- `ZONOMI_ENCRYPT_KEY`: 32-byte encryption key for decrypting API key
- `RUN_ONCE`: Set to "true" to run the task once and exit (for testing)

## Encryption of ZONOMI_API_KEY
The API key can be encrypted using AES-256-GCM for security. Use the following Go code to encrypt your API key:

```go
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run encrypt.go <api_key> <encrypt_key>")
		os.Exit(1)
	}
	apiKey := []byte(os.Args[1])
	encryptKey := []byte(os.Args[2])
	if len(encryptKey) != 32 {
		fmt.Println("Encryption key must be 32 bytes")
		os.Exit(1)
	}

	encrypted, err := encrypt(apiKey, encryptKey)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Encrypted API Key:", encrypted)
}

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
```

### How to Encrypt:
1. Save the above code as `encrypt.go`.
2. Run: `go run encrypt.go "your-api-key" "your-32-byte-encrypt-key"`.
3. Use the output as `ZONOMI_API_KEY` in your environment.
4. Set `ZONOMI_API_ENCRYPTED=true` and `ZONOMI_ENCRYPT_KEY=your-32-byte-encrypt-key`.

**Note:** The encryption key should be securely stored (e.g., in Docker secrets). Base64 is used for encoding the ciphertext.

## Building and Running in Docker
1. Build the image:
   ```
   docker build -t zonocaller .
   ```
2. Run with volume for persistence and secrets:
   ```
   mkdir data
   docker run --rm -v $(pwd)/data:/app/data \
     -e ZONOMI_API_KEY=your-encrypted-key \
     -e ZONOMI_API_ENCRYPTED=true \
     -e ZONOMI_ENCRYPT_KEY=your-encrypt-key \
     zonocaller
   ```

For testing in run-once mode:
```
docker run --rm -v $(pwd)/data:/app/data \
  -e RUN_ONCE=true \
  -e ZONOMI_API_KEY=your-encrypted-key \
  -e ZONOMI_API_ENCRYPTED=true \
  -e ZONOMI_ENCRYPT_KEY=your-encrypt-key \
  zonocaller
```

## Testing
Run unit tests:
```
go test ./...
```

## Dependencies
- github.com/go-co-op/gocron/v2
- github.com/cenkalti/backoff/v4
- github.com/stretchr/testify (for tests)

Install:
```
go get github.com/go-co-op/gocron/v2
go get github.com/cenkalti/backoff/v4
go get github.com/stretchr/testify
```

### Additional Notes
Not all APIs are supported
No support for mulitple DNSes (myhost.com, sub.myhost.com, anothersub.myhost.com)
-

#### Additional Docker help
Building and Running in Docker

```bash
docker build -t zonocaller .
```

Run the container with a volume for persistence and secure API key:

```bash
mkdir data
docker run --rm -v $(pwd)/data:/app/data \
  -e ZONOMI_API_KEY=your-actual-api-key \
  zonocaller
```

Replace your-actual-api-key with your real Zonomi API key.
The -v flag mounts ./data to /app/data for ip_log.txt persistence.


Override other environment variables if needed:

```bash
docker run --rm -v $(pwd)/data:/app/data \
  -e ZONOMI_API_KEY=your-actual-api-key \
  -e TIMEZONE=UTC \
  -e SCHEDULE_TIME=23:00 \
  zonocaller
```


#### Persistent Logging

IP Log File: Appended entries in data/ip_log.txt (e.g., IP: 203.0.113.1, Timestamp: 2025-08-30T23:59:00Z).
Application Logs: Sent to stdout and captured by Docker. Persist logs using a logging driver:

```bash
docker run --rm -v $(pwd)/data:/app/data \
  -e ZONOMI_API_KEY=your-actual-api-key \
  --log-driver=json-file \
  --log-opt max-size=10m \
  zonocaller
```
