// Package config loads runtime configuration from /etc/vibhost/agent.conf.
// Format is a simple KEY=VALUE file, one entry per line. Lines starting with #
// and blank lines are ignored.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds all runtime settings for vh-agent.
type Config struct {
	ListenAddr     string // host:port for the gRPC listener (required)
	WorkdirDefault string // default working directory for Exec requests
	LogLevel       string // zerolog level: debug|info|warn|error
}

// Load reads and parses the configuration file at path. Missing optional
// values are filled with defaults. Returns an error when the file cannot be
// read or when mandatory keys are absent.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	cfg := &Config{
		WorkdirDefault: "/home/appuser",
		LogLevel:       "info",
	}

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			return nil, fmt.Errorf("%s:%d: malformed line (expected KEY=VALUE)", path, lineNo)
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		switch key {
		case "GRPC_LISTEN_ADDR":
			cfg.ListenAddr = value
		case "WORKDIR_DEFAULT":
			if value != "" {
				cfg.WorkdirDefault = value
			}
		case "LOG_LEVEL":
			if value != "" {
				cfg.LogLevel = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	if cfg.ListenAddr == "" {
		return nil, errors.New("GRPC_LISTEN_ADDR is required in agent.conf")
	}
	return cfg, nil
}
