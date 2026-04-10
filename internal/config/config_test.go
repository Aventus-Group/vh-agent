package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ReadsConfigAndToken(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "agent.conf")
	tokenPath := filepath.Join(dir, "agent.token")

	confContent := "CONTAINER_ID=abc-123\nBOOTSTRAP_URL=https://bootstrap.vibhost.com\n"
	if err := os.WriteFile(confPath, []byte(confContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte("secret-token-42\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(confPath, tokenPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ContainerID != "abc-123" {
		t.Errorf("ContainerID = %q, want %q", cfg.ContainerID, "abc-123")
	}
	if cfg.BootstrapURL != "https://bootstrap.vibhost.com" {
		t.Errorf("BootstrapURL = %q, want %q", cfg.BootstrapURL, "https://bootstrap.vibhost.com")
	}
	if cfg.Token != "secret-token-42" {
		t.Errorf("Token = %q, want %q (whitespace should be trimmed)", cfg.Token, "secret-token-42")
	}
}

func TestLoad_MissingConfigFile(t *testing.T) {
	_, err := Load("/nonexistent/agent.conf", "/nonexistent/agent.token")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoad_MissingRequiredField(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "agent.conf")
	tokenPath := filepath.Join(dir, "agent.token")

	// Missing CONTAINER_ID
	if err := os.WriteFile(confPath, []byte("BOOTSTRAP_URL=https://x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte("tok"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(confPath, tokenPath)
	if err == nil {
		t.Fatal("expected error for missing CONTAINER_ID")
	}
}
