package fetcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Drakx/ZonoCaller/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {

	cfg := config.Config{
		APIURL:       "https://api.ipify.org?format=json",
		ZonomiAPIURL: "https://zonomi.com/app/dns/dyndns.jsp",
		OutputFile:   filepath.Join(t.TempDir(), "ip_log.txt"),
		MaxRetries:   3,
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	assert.NotNil(t, f.logger, "Logger should be initialized")
	assert.NotNil(t, f.client, "HTTP client should be initialized")
	assert.Equal(t, cfg, f.config, "Config should be set")
	assert.Equal(t, 10*time.Second, f.client.Timeout, "Client timeout should be 10 seconds")
	assert.NotNil(t, f.retryBackoff, "Retry backoff should be initialized")
}

func TestFetchIP_FirstRun(t *testing.T) {

	// Mock ipify server
	ipifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ip":"192.168.1.1"}`))
	}))
	defer ipifyServer.Close()

	// Mock Zonomi server
	zonomiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, []string{"test.host1", "test.host2"}, r.URL.Query().Get("name"))
		assert.Equal(t, "A", r.URL.Query().Get("type"))
		assert.Equal(t, "192.168.1.1", r.URL.Query().Get("value"))
		assert.Equal(t, "test-key", r.URL.Query().Get("api_key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer zonomiServer.Close()

	// Create temp output file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")

	// Config
	cfg := config.Config{
		APIURL:       ipifyServer.URL,
		ZonomiAPIURL: zonomiServer.URL,
		OutputFile:   outputFile,
		MaxRetries:   1,
		ZonomiHosts:  []string{"test.host1", "test.host2"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	// Run FetchIP
	err := f.FetchIP(context.Background())
	require.NoError(t, err)

	// Check log file
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	var entry IPLogEntry
	err = json.Unmarshal([]byte(strings.Split(string(data), "\n")[0]), &entry)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", entry.IP)
	assert.NotEmpty(t, entry.Timestamp)
}

func TestFetchIP_NoIPChange(t *testing.T) {

	// Mock ipify server
	ipifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ip":"192.168.1.1"}`))
	}))
	defer ipifyServer.Close()

	// Mock Zonomi server (should not be called)
	zonomiCalled := false
	zonomiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zonomiCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer zonomiServer.Close()

	// Create temp output file with existing IP
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")
	entry := IPLogEntry{IP: "192.168.1.1", Timestamp: "2025-08-30T12:00:00Z"}
	data, _ := json.Marshal(entry)
	err := os.WriteFile(outputFile, append(data, '\n'), 0644)
	require.NoError(t, err)

	// Config
	cfg := config.Config{
		APIURL:       ipifyServer.URL,
		ZonomiAPIURL: zonomiServer.URL,
		OutputFile:   outputFile,
		MaxRetries:   1,
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	// Run FetchIP
	err = f.FetchIP(context.Background())
	require.NoError(t, err)

	// Verify Zonomi was not called
	assert.False(t, zonomiCalled, "Zonomi API should not be called when IP is unchanged")

	// Check log file
	data, err = os.ReadFile(outputFile)
	require.NoError(t, err)

	lines := strings.Split(string(data), "\n")
	var lastEntry IPLogEntry
	err = json.Unmarshal([]byte(lines[len(lines)-2]), &lastEntry) // Last line is empty
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", lastEntry.IP)
}

func TestFetchIP_IPChange(t *testing.T) {

	// Mock ipify server
	ipifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ip":"192.168.1.2"}`))
	}))
	defer ipifyServer.Close()

	// Mock Zonomi server
	zonomiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, []string{"test.host1", "test.host2"}, r.URL.Query().Get("name"))
		assert.Equal(t, "A", r.URL.Query().Get("type"))
		assert.Equal(t, "192.168.1.2", r.URL.Query().Get("value"))
		assert.Equal(t, "test-key", r.URL.Query().Get("api_key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer zonomiServer.Close()

	// Create temp output file with existing IP
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")
	entry := IPLogEntry{IP: "192.168.1.1", Timestamp: "2025-08-30T12:00:00Z"}
	data, _ := json.Marshal(entry)
	err := os.WriteFile(outputFile, append(data, '\n'), 0644)
	require.NoError(t, err)

	// Config
	cfg := config.Config{
		APIURL:       ipifyServer.URL,
		ZonomiAPIURL: zonomiServer.URL,
		OutputFile:   outputFile,
		MaxRetries:   1,
		ZonomiHosts:  []string{"test.host1", "test.host2"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	// Run FetchIP
	err = f.FetchIP(context.Background())
	require.NoError(t, err)

	// Check log file
	data, err = os.ReadFile(outputFile)
	require.NoError(t, err)

	lines := strings.Split(string(data), "\n")
	var lastEntry IPLogEntry
	err = json.Unmarshal([]byte(lines[len(lines)-2]), &lastEntry) // Last line is empty
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.2", lastEntry.IP)
}

func TestFetchIP_MultipleHosts(t *testing.T) {

	// Mock ipify server
	ipifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ip":"192.168.1.1"}`))
	}))
	defer ipifyServer.Close()

	// Mock Zonomi server, track calls
	hostsCalled := make(map[string]int)
	zonomiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.URL.Query().Get("name")
		hostsCalled[host]++
		assert.Equal(t, "A", r.URL.Query().Get("type"))
		assert.Equal(t, "192.168.1.1", r.URL.Query().Get("value"))
		assert.Equal(t, "test-key", r.URL.Query().Get("api_key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer zonomiServer.Close()

	// Create temp output file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")

	// Config with multiple hosts
	cfg := config.Config{
		APIURL:       ipifyServer.URL,
		ZonomiAPIURL: zonomiServer.URL,
		OutputFile:   outputFile,
		MaxRetries:   1,
		ZonomiHosts:  []string{"host1.example.com", "host2.example.com"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	// Run FetchIP
	err := f.FetchIP(context.Background())
	require.NoError(t, err)

	// Verify each host was called once
	assert.Equal(t, 1, hostsCalled["host1.example.com"], "host1.example.com should be called once")
	assert.Equal(t, 1, hostsCalled["host2.example.com"], "host2.example.com should be called once")

	// Check log file
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	var entry IPLogEntry
	err = json.Unmarshal([]byte(strings.Split(string(data), "\n")[0]), &entry)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", entry.IP)
}

func TestReadLastIP_EmptyFile(t *testing.T) {

	// Create empty temp file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")
	err := os.WriteFile(outputFile, []byte(""), 0644)
	require.NoError(t, err)

	// Config
	cfg := config.Config{
		OutputFile:   outputFile,
		ZonomiAPIURL: "https://zonomi.com/app/dns/dyndns.jsp",
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	// Read last IP
	ip, err := f.readLastIP()
	require.NoError(t, err)
	assert.Empty(t, ip, "Empty file should return empty IP")
}

func TestReadLastIP_ValidFile(t *testing.T) {

	// Create temp file with JSON IP log
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")
	entry := IPLogEntry{IP: "192.168.1.1", Timestamp: "2025-08-30T12:00:00Z"}
	data, _ := json.Marshal(entry)
	err := os.WriteFile(outputFile, append(data, '\n'), 0644)
	require.NoError(t, err)

	// Config
	cfg := config.Config{
		OutputFile:   outputFile,
		ZonomiAPIURL: "https://zonomi.com/app/dns/dyndns.jsp",
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	// Read last IP
	ip, err := f.readLastIP()
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", ip)
}

func TestReadLastIP_NonExistentFile(t *testing.T) {

	// Config with non-existent file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")

	cfg := config.Config{
		OutputFile:   outputFile,
		ZonomiAPIURL: "https://zonomi.com/app/dns/dyndns.jsp",
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	// Read last IP
	ip, err := f.readLastIP()
	require.NoError(t, err)
	assert.Empty(t, ip, "Non-existent file should return empty IP")
}

func TestUpdateZonomiDNS_Success(t *testing.T) {

	// Mock Zonomi server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, []string{"test.host"}, r.URL.Query().Get("name"))
		assert.Equal(t, "A", r.URL.Query().Get("type"))
		assert.Equal(t, "192.168.1.1", r.URL.Query().Get("value"))
		assert.Equal(t, "test-key", r.URL.Query().Get("api_key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Config
	cfg := config.Config{
		ZonomiAPIURL: server.URL,
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
		MaxRetries:   1,
	}

	f := New(cfg)

	// Update DNS
	err := f.updateZonomiDNS("192.168.1.1")
	require.NoError(t, err)
}

func TestUpdateZonomiDNS_Failure(t *testing.T) {

	// Mock Zonomi server with failure
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`<?xml version="1.0"?><!DOCTYPE html [<!ENTITY nbsp "&#160;"><!ENTITY trade "&#8482;"><!ENTITY copy "&#169;">]><error>ERROR: Invalid api_key.</error>`))
	}))
	defer server.Close()

	// Config
	cfg := config.Config{
		ZonomiAPIURL: server.URL,
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
		MaxRetries:   1,
	}

	f := New(cfg)

	// Update DNS
	err := f.updateZonomiDNS("192.168.1.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `errors updating hosts`)
}

func TestUpdateZonomiDNS_Retry(t *testing.T) {

	// Mock Zonomi server with retryable failure
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Config
	cfg := config.Config{
		ZonomiAPIURL: server.URL,
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
		MaxRetries:   2,
	}

	f := New(cfg)

	// Update DNS
	err := f.updateZonomiDNS("192.168.1.1")
	require.NoError(t, err)
	assert.Equal(t, 2, attempts, "Should retry once before succeeding")
}

func TestAppendIP(t *testing.T) {

	// Create temp output file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")

	// Config
	cfg := config.Config{
		OutputFile:   outputFile,
		ZonomiAPIURL: "https://zonomi.com/app/dns/dyndns.jsp",
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
	}

	f := New(cfg)

	// Append IP
	err := f.appendIP("192.168.1.1")
	require.NoError(t, err)

	// Check file contents
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	var entry IPLogEntry
	err = json.Unmarshal([]byte(strings.Split(string(data), "\n")[0]), &entry)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", entry.IP)
	assert.NotEmpty(t, entry.Timestamp)
}
