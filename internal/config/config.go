package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config holds runtime configuration loaded from /etc/vibhost/agent.conf and agent.token.
type Config struct {
	ContainerID  string
	BootstrapURL string
	Token        string
}

// Load reads config from confPath (env-file format: KEY=VALUE) and token from tokenPath.
// Both files must exist and be readable.
func Load(confPath, tokenPath string) (*Config, error) {
	confFile, err := os.Open(confPath)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", confPath, err)
	}
	defer confFile.Close()

	cfg := &Config{}
	scanner := bufio.NewScanner(confFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "CONTAINER_ID":
			cfg.ContainerID = value
		case "BOOTSTRAP_URL":
			cfg.BootstrapURL = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan config: %w", err)
	}

	if cfg.ContainerID == "" {
		return nil, fmt.Errorf("config %s: CONTAINER_ID is required", confPath)
	}
	if cfg.BootstrapURL == "" {
		return nil, fmt.Errorf("config %s: BOOTSTRAP_URL is required", confPath)
	}

	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("read token %s: %w", tokenPath, err)
	}
	cfg.Token = strings.TrimSpace(string(tokenBytes))
	if cfg.Token == "" {
		return nil, fmt.Errorf("token file %s is empty", tokenPath)
	}

	return cfg, nil
}

// Redacted returns a loggable summary without the token.
func (c *Config) Redacted() string {
	return fmt.Sprintf("container_id=%s bootstrap_url=%s token=<%d bytes>",
		c.ContainerID, c.BootstrapURL, len(c.Token))
}
