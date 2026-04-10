package wg

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Patcher applies WireGuard endpoint changes live and persists them to the config file.
type Patcher struct {
	ConfigPath string
	Interface  string
	Exec       func(name string, args ...string) error
}

// NewPatcher returns a patcher wired to os/exec.
func NewPatcher(configPath, iface string) *Patcher {
	return &Patcher{
		ConfigPath: configPath,
		Interface:  iface,
		Exec: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s: %w (output: %s)", name, err, string(out))
			}
			return nil
		},
	}
}

// UpdateEndpoint runs `wg set <iface> peer <pubkey> endpoint <new>` and rewrites
// the Endpoint= line in the config file for persistence through restarts.
func (p *Patcher) UpdateEndpoint(pubkey, newEndpoint string) error {
	if err := p.Exec("wg", "set", p.Interface, "peer", pubkey, "endpoint", newEndpoint); err != nil {
		return fmt.Errorf("wg set: %w", err)
	}
	return p.rewriteConfigEndpoint(newEndpoint)
}

// ReadCurrentEndpoint returns the current Endpoint= value from the config file.
func (p *Patcher) ReadCurrentEndpoint() (string, error) {
	f, err := os.Open(p.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("open config: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Endpoint") {
			_, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			return strings.TrimSpace(value), nil
		}
	}
	return "", scanner.Err()
}

// rewriteConfigEndpoint rewrites the Endpoint= line in place.
// Simple approach: read, replace first matching line, write atomically via temp file.
func (p *Patcher) rewriteConfigEndpoint(newEndpoint string) error {
	data, err := os.ReadFile(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Endpoint") {
			// Preserve indentation
			indent := line[:len(line)-len(trimmed)]
			lines[i] = indent + "Endpoint = " + newEndpoint
			break
		}
	}
	newContent := strings.Join(lines, "\n")

	tmp := p.ConfigPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(newContent), 0o600); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, p.ConfigPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
