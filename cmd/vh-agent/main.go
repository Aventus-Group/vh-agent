package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/Aventus-Group/vh-agent/internal/bootstrap"
	"github.com/Aventus-Group/vh-agent/internal/config"
	"github.com/Aventus-Group/vh-agent/internal/metrics"
	"github.com/Aventus-Group/vh-agent/internal/updater"
	"github.com/Aventus-Group/vh-agent/internal/wg"
)

// Version is injected at build time via -ldflags "-X main.Version=1.0.0".
var Version = "dev"

const (
	defaultConfPath = "/etc/vibhost/agent.conf"
	// defaultTokenPath is a filesystem path, not a credential itself.
	defaultTokenPath  = "/etc/vibhost/agent.token" //nolint:gosec // path constant, not a hardcoded secret
	defaultBinaryPath = "/usr/local/bin/vh-agent"
	defaultWGConfig   = "/root/.vibhost/wg/vhnet0.conf"
	defaultInterface  = "vhnet0"
	initialInterval   = 300 * time.Second
	httpTimeout       = 15 * time.Second
	minBackoff        = 30 * time.Second
	maxBackoff        = 300 * time.Second
)

func main() {
	confPath := flag.String("config", envOrDefault("VH_AGENT_CONFIG", defaultConfPath), "path to agent.conf")
	tokenPath := flag.String("token", envOrDefault("VH_AGENT_TOKEN_FILE", defaultTokenPath), "path to agent.token")
	binaryPath := flag.String("binary", envOrDefault("VH_AGENT_BINARY", defaultBinaryPath), "path to vh-agent binary (for self-update)")
	wgConfPath := flag.String("wg-config", envOrDefault("VH_AGENT_WG_CONFIG", defaultWGConfig), "path to WireGuard config file")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("vh-agent starting version=%s", Version)

	cfg, err := config.Load(*confPath, *tokenPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("config loaded: %s", cfg.Redacted())

	client := bootstrap.NewClient(cfg.BootstrapURL, cfg.Token, httpTimeout)
	collector := metrics.DefaultCollector()
	patcher := wg.NewPatcher(*wgConfPath, defaultInterface)
	upd := updater.NewUpdater(*binaryPath)

	// Initial GetConfig at startup — warm up and verify token
	initialCfg, err := client.GetConfig(cfg.ContainerID)
	if err != nil {
		if errors.Is(err, bootstrap.ErrUnauthorized) {
			log.Fatalf("initial GetConfig: unauthorized, stopping")
		}
		log.Printf("initial GetConfig failed: %v (continuing, will retry)", err)
	} else {
		applyConfigChanges(initialCfg, patcher, upd)
	}

	// Take first CPU snapshot to prime the delta (next Collect will have valid %)
	_, _ = collector.Collect()

	interval := initialInterval
	if initialCfg != nil && initialCfg.HeartbeatIntervalSec > 0 {
		interval = time.Duration(initialCfg.HeartbeatIntervalSec) * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("signal received, shutting down")
		cancel()
	}()

	startTime := time.Now()
	backoffAttempt := 0
	ticker := time.NewTimer(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("main loop exiting")
			return
		case <-ticker.C:
		}

		snap, err := collector.Collect()
		if err != nil {
			log.Printf("collect metrics: %v", err)
			snap = &metrics.Snapshot{} // send empty metrics, still heartbeat
		}

		req := &bootstrap.HeartbeatRequest{
			ContainerID:       cfg.ContainerID,
			AgentVersion:      Version,
			UptimeSec:         int64(time.Since(startTime).Seconds()),
			CPUPct:            snap.CPUPct,
			MemUsedMB:         snap.MemUsedMB,
			MemTotalMB:        snap.MemTotalMB,
			DiskUsedGB:        snap.DiskUsedGB,
			DiskTotalGB:       snap.DiskTotalGB,
			Load1m:            snap.Load1m,
			WGHandshakeAgeSec: snap.WGHandshakeAgeSec,
			WGEndpoint:        snap.WGEndpoint,
		}

		resp, err := client.Heartbeat(req)
		if err != nil {
			if errors.Is(err, bootstrap.ErrUnauthorized) {
				log.Printf("heartbeat 401, stopping agent")
				stopSystemdAndExit()
				return
			}
			backoffAttempt++
			delay := backoffDelay(backoffAttempt)
			log.Printf("heartbeat failed (attempt %d): %v, retrying in %v", backoffAttempt, err, delay)
			ticker.Reset(delay)
			continue
		}
		backoffAttempt = 0

		applyConfigChanges(resp, patcher, upd)

		if resp.HeartbeatIntervalSec > 0 {
			interval = time.Duration(resp.HeartbeatIntervalSec) * time.Second
		}
		ticker.Reset(interval)
	}
}

func applyConfigChanges(resp *bootstrap.ConfigResponse, patcher *wg.Patcher, upd *updater.Updater) {
	if resp == nil {
		return
	}

	// WG endpoint sync
	if resp.WG.RouterEndpoint != "" && resp.WG.RouterPublicKey != "" {
		curr, err := patcher.ReadCurrentEndpoint()
		if err != nil {
			log.Printf("read current endpoint: %v", err)
		} else if curr != resp.WG.RouterEndpoint {
			log.Printf("wg endpoint changed: %s → %s", curr, resp.WG.RouterEndpoint)
			if err := patcher.UpdateEndpoint(resp.WG.RouterPublicKey, resp.WG.RouterEndpoint); err != nil {
				log.Printf("update endpoint: %v", err)
			} else {
				log.Printf("wg endpoint updated successfully")
			}
		}
	}

	// Self-update
	if resp.Agent.LatestVersion != "" && resp.Agent.LatestVersion != Version {
		log.Printf("new version available: %s → %s", Version, resp.Agent.LatestVersion)
		if err := upd.Update(resp.Agent.DownloadURL, resp.Agent.SHA256); err != nil {
			log.Printf("self-update failed: %v", err)
		} else {
			log.Printf("self-update successful, restarting via systemd")
			restartSystemdAndExit()
		}
	}
}

func stopSystemdAndExit() {
	cmd := exec.Command("systemctl", "stop", "vibhost-agent")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("systemctl stop failed: %v (output: %s)", err, out)
	}
	os.Exit(0)
}

func restartSystemdAndExit() {
	cmd := exec.Command("systemctl", "restart", "vibhost-agent")
	if err := cmd.Start(); err != nil {
		log.Printf("systemctl restart failed: %v", err)
	}
	// Systemd will kill us shortly; exit cleanly so it doesn't think we crashed
	os.Exit(0)
}

func backoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exp := math.Pow(2, float64(attempt-1))
	d := time.Duration(exp) * minBackoff
	if d > maxBackoff {
		d = maxBackoff
	}
	return d
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
