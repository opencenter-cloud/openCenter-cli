/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package observability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/syslog"
	"net/http"
	"time"
)

// LogShipper defines the interface for shipping logs to external systems
// Requirements: 12.1
type LogShipper interface {
	Ship(entry LogEntry) error
	Close() error
}

// SyslogShipper ships logs to a syslog server
// Requirements: 12.1
type SyslogShipper struct {
	writer *syslog.Writer
}

// NewSyslogShipper creates a new syslog shipper
// network can be "tcp", "udp", or "" for local syslog
// address is the syslog server address (e.g., "localhost:514")
func NewSyslogShipper(network, address string) (*SyslogShipper, error) {
	writer, err := syslog.Dial(network, address, syslog.LOG_INFO|syslog.LOG_USER, "openCenter")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to syslog: %w", err)
	}

	return &SyslogShipper{
		writer: writer,
	}, nil
}

// Ship sends a log entry to syslog
func (s *SyslogShipper) Ship(entry LogEntry) error {
	// Format the log entry as JSON
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Send to syslog based on level
	switch entry.Level {
	case "DEBUG":
		return s.writer.Debug(string(jsonBytes))
	case "INFO":
		return s.writer.Info(string(jsonBytes))
	case "WARN":
		return s.writer.Warning(string(jsonBytes))
	case "ERROR":
		return s.writer.Err(string(jsonBytes))
	default:
		return s.writer.Info(string(jsonBytes))
	}
}

// Close closes the syslog connection
func (s *SyslogShipper) Close() error {
	if s.writer != nil {
		return s.writer.Close()
	}
	return nil
}

// LokiShipper ships logs to a Loki server
// Requirements: 12.1
type LokiShipper struct {
	url    string
	client *http.Client
}

// LokiPushRequest represents a Loki push request
type LokiPushRequest struct {
	Streams []LokiStream `json:"streams"`
}

// LokiStream represents a Loki log stream
type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// NewLokiShipper creates a new Loki shipper
// url is the Loki push API endpoint (e.g., "http://localhost:3100/loki/api/v1/push")
func NewLokiShipper(url string) *LokiShipper {
	return &LokiShipper{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Ship sends a log entry to Loki
func (l *LokiShipper) Ship(entry LogEntry) error {
	// Convert log entry to Loki format
	labels := map[string]string{
		"level": entry.Level,
		"app":   "openCenter",
	}

	// Add correlation ID as label if present
	if entry.CorrelationID != "" {
		labels["correlation_id"] = entry.CorrelationID
	}

	// Add fields as labels
	if entry.Fields != nil {
		for key, value := range entry.Fields {
			// Convert value to string
			labels[key] = fmt.Sprintf("%v", value)
		}
	}

	// Format log line as JSON
	logLine, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Create Loki push request
	pushReq := LokiPushRequest{
		Streams: []LokiStream{
			{
				Stream: labels,
				Values: [][]string{
					{
						// Timestamp in nanoseconds
						fmt.Sprintf("%d", time.Now().UnixNano()),
						string(logLine),
					},
				},
			},
		},
	}

	// Marshal push request
	reqBody, err := json.Marshal(pushReq)
	if err != nil {
		return fmt.Errorf("failed to marshal Loki push request: %w", err)
	}

	// Send HTTP POST request
	resp, err := l.client.Post(l.url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to send log to Loki: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Loki returned error status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close closes the Loki shipper (no-op for HTTP client)
func (l *LokiShipper) Close() error {
	return nil
}

// MultiShipper ships logs to multiple destinations
type MultiShipper struct {
	shippers []LogShipper
}

// NewMultiShipper creates a new multi-shipper
func NewMultiShipper(shippers ...LogShipper) *MultiShipper {
	return &MultiShipper{
		shippers: shippers,
	}
}

// Ship sends a log entry to all configured shippers
func (m *MultiShipper) Ship(entry LogEntry) error {
	var errs []error
	for _, shipper := range m.shippers {
		if err := shipper.Ship(entry); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to ship to %d destinations: %v", len(errs), errs)
	}

	return nil
}

// Close closes all shippers
func (m *MultiShipper) Close() error {
	var errs []error
	for _, shipper := range m.shippers {
		if err := shipper.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close %d shippers: %v", len(errs), errs)
	}

	return nil
}
