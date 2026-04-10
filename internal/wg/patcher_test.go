package wg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatcher_UpdateEndpoint_ReplacesInConfig(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "vhnet0.conf")
	original := `[Interface]
PrivateKey = PRIV
Address = 10.10.75.42/32

[Peer]
PublicKey = ROUTER_PK
Endpoint = 78.47.77.236:48720
AllowedIPs = 0.0.0.0/0
`
	if err := os.WriteFile(confPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotCmd []string
	p := &Patcher{
		ConfigPath: confPath,
		Interface:  "vhnet0",
		Exec: func(name string, args ...string) error {
			gotCmd = append([]string{name}, args...)
			return nil
		},
	}

	err := p.UpdateEndpoint("ROUTER_PK", "1.2.3.4:48720")
	if err != nil {
		t.Fatalf("UpdateEndpoint: %v", err)
	}

	// Verify wg set was called correctly
	expected := []string{"wg", "set", "vhnet0", "peer", "ROUTER_PK", "endpoint", "1.2.3.4:48720"}
	if len(gotCmd) != len(expected) {
		t.Fatalf("cmd = %v, want %v", gotCmd, expected)
	}
	for i := range expected {
		if gotCmd[i] != expected[i] {
			t.Errorf("cmd[%d] = %q, want %q", i, gotCmd[i], expected[i])
		}
	}

	// Verify config file was updated for persistence
	updated, _ := os.ReadFile(confPath)
	if !strings.Contains(string(updated), "Endpoint = 1.2.3.4:48720") {
		t.Errorf("config not updated:\n%s", updated)
	}
	if strings.Contains(string(updated), "Endpoint = 78.47.77.236:48720") {
		t.Errorf("old endpoint still present:\n%s", updated)
	}
}

func TestPatcher_UpdateEndpoint_NoOpIfSame(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "vhnet0.conf")
	conf := "[Peer]\nEndpoint = 1.2.3.4:48720\n"
	if err := os.WriteFile(confPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}

	execCalled := false
	p := &Patcher{
		ConfigPath: confPath,
		Interface:  "vhnet0",
		Exec: func(name string, args ...string) error {
			execCalled = true
			return nil
		},
	}

	// NoOp check via ReadCurrentEndpoint
	curr, err := p.ReadCurrentEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	if curr != "1.2.3.4:48720" {
		t.Errorf("curr = %q", curr)
	}
	if execCalled {
		t.Error("ReadCurrentEndpoint should not exec")
	}
}
