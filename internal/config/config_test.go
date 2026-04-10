package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ParsesAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.conf")
	body := "GRPC_LISTEN_ADDR=10.10.75.42:50051\nWORKDIR_DEFAULT=/srv/app\nLOG_LEVEL=debug\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != "10.10.75.42:50051" {
		t.Errorf("ListenAddr: got %q", cfg.ListenAddr)
	}
	if cfg.WorkdirDefault != "/srv/app" {
		t.Errorf("WorkdirDefault: got %q", cfg.WorkdirDefault)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q", cfg.LogLevel)
	}
}

func TestLoad_AppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.conf")
	if err := os.WriteFile(path, []byte("GRPC_LISTEN_ADDR=127.0.0.1:50051\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkdirDefault != "/home/appuser" {
		t.Errorf("default workdir: got %q", cfg.WorkdirDefault)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default log level: got %q", cfg.LogLevel)
	}
}

func TestLoad_RequiresListenAddr(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.conf")
	if err := os.WriteFile(path, []byte("LOG_LEVEL=info\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error when GRPC_LISTEN_ADDR missing")
	}
}

func TestLoad_IgnoresCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.conf")
	body := "# comment\n\nGRPC_LISTEN_ADDR=127.0.0.1:50051\n# trailing comment\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:50051" {
		t.Errorf("got %q", cfg.ListenAddr)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	if _, err := Load("/nonexistent/agent.conf"); err == nil {
		t.Fatal("expected error for missing file")
	}
}
