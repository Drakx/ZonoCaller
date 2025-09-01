package fetcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Drakx/ZonoCaller/internal/config"
	"github.com/cenkalti/backoff/v4"
)

// IPResponse represents the ipify API response
type IPResponse struct {
	IP string `json:"ip"`
}

// IPLogEntry represents a single entry in the ip log file
type IPLogEntry struct {
	IP        string `json:"ip"`
	Timestamp string `json:timestamp`
}

// FetcherInterface defines the interface for Fetcher
type FetcherInterface interface {
	FetchIP(context.Context) error
}

// Fetcher handles IP fetching, comparison, and DNS updates
type Fetcher struct {
	logger       *slog.Logger
	client       *http.Client
	config       config.Config
	retryBackoff backoff.BackOff
}

// New creates a new Fetcher instance
func New(cfg config.Config) *Fetcher {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))

	client := &http.Client{Timeout: 10 * time.Second}

	return &Fetcher{
		logger: logger,
		client: client,
		config: cfg,
		retryBackoff: backoff.NewExponentialBackOff(
			backoff.WithInitialInterval(1*time.Second),
			backoff.WithMaxInterval(10*time.Second),
			backoff.WithMaxElapsedTime(30*time.Second),
		),
	}
}

// FetchIP retrieves the public IP, checks for changes, and updates DNS if needed.
func (f *Fetcher) FetchIP(_ context.Context) error {

	f.logger.Info("Fetching public IP", "url", f.config.APIURL)

	// Fetch current IP
	newIP, err := f.fetchCurrentIP()
	if err != nil {
		f.logger.Error("Failed to fetch IP", "error", err)
		return err
	}

	// Read last IP from log file
	lastIP, err := f.readLastIP()
	if err != nil {
		f.logger.Warn("Failed to read last IP, treating as first run", "error", err)
	}

	// Append new IP to log file
	if err := f.appendIP(newIP); err != nil {
		f.logger.Error("Failed to append IP", "error", err)
		return err
	}

	// Check if IP has changed or is first run
	if lastIP == "" || lastIP != newIP {
		f.logger.Info("IP changed or first run", "last_ip", lastIP, "new_ip", newIP)

		if err := f.updateZonomiDNS(newIP); err != nil {
			f.logger.Error("Failed to update Zonomi DNS", "error", err)
			return err
		}

		f.logger.Info("Zonomi DNS updated", "ip", newIP, "hosts", f.config.ZonomiHosts)

		return nil
	}

	f.logger.Info("IP unchanged, skipping DNS update", "ip", newIP)

	return nil
}

// fetchCurrentIP retrieves the current public IP from the ipify API.
func (f *Fetcher) fetchCurrentIP() (string, error) {

	var ipResp IPResponse
	operation := func() error {
		resp, err := f.client.Get(f.config.APIURL)
		if err != nil {
			return fmt.Errorf("HTTP request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %s", resp.Status)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		return json.Unmarshal(body, &ipResp)
	}

	err := backoff.RetryNotify(operation, backoff.WithMaxRetries(f.retryBackoff, uint64(f.config.MaxRetries)),
		func(err error, duration time.Duration) {
			f.logger.Warn("Retrying IP fetch", "error", err, "retry_after", duration)
		})
	if err != nil {
		return "", err
	}

	return ipResp.IP, nil
}

// readLastIP reads the last IP from the log file
func (f *Fetcher) readLastIP() (string, error) {

	file, err := os.Open(f.config.OutputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var lastIP string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var lastEntry IPLogEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &lastEntry); err == nil {
			lastIP = lastEntry.IP
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read log file: %w", err)
	}

	return lastIP, nil
}

// updateZonomiDNS calls the DNS update API for each host
func (f *Fetcher) updateZonomiDNS(ip string) error {

	var errs []error
	for _, host := range f.config.ZonomiHosts {
		query := url.Values{}
		query.Set("name", host)
		query.Set("value", ip)
		query.Set("type", "A")
		query.Set("api_key", f.config.ZonomiAPIKey)

		urlStr := f.config.ZonomiAPIURL + "?" + query.Encode()

		f.logger.Info("Calling Zonomi API", "host", host, "url", urlStr)

		operation := func() error {

			resp, err := f.client.Get(urlStr)
			if err != nil {
				return fmt.Errorf("Zonomi API request failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("unexpected status: %s, body: %s", resp.Status, string(body))
			}

			return nil
		}

		err := backoff.RetryNotify(operation, backoff.WithMaxRetries(f.retryBackoff, uint64(f.config.MaxRetries)),
			func(err error, d time.Duration) {
				f.logger.Warn("Retrying Zonomi API", "host", host, "error", err, "retry_after", d)
			})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed for host %s: %w", host, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors updating hosts: %v", errs)
	}

	return nil
}

// appendIP appends the IP and timestamp to the output file
func (f *Fetcher) appendIP(ip string) error {

	// Open or crate the file if needed
	file, err := os.OpenFile(f.config.OutputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	entry := IPLogEntry{
		IP:        ip,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}
