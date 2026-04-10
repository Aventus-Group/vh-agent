// Command vh-agent starts the VibHost resident gRPC daemon.
package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Aventus-Group/vh-agent/internal/config"
	"github.com/Aventus-Group/vh-agent/internal/server"
	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Version is overridden at build time via -ldflags.
var Version = "dev"

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	configPath := os.Getenv("VH_AGENT_CONFIG")
	if configPath == "" {
		configPath = "/etc/vibhost/agent.conf"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", configPath).Msg("load config")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	log.Info().
		Str("version", Version).
		Str("listen", cfg.ListenAddr).
		Str("workdir_default", cfg.WorkdirDefault).
		Msg("vh-agent starting")

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("listen")
	}

	srv := server.New(Version, cfg.WorkdirDefault)

	// Signal the manager once the listener is ready.
	if _, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		log.Warn().Err(err).Msg("sd_notify ready failed (non-systemd environment?)")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info().Stringer("signal", sig).Msg("shutting down")
		if _, err := daemon.SdNotify(false, daemon.SdNotifyStopping); err != nil {
			log.Warn().Err(err).Msg("sd_notify stopping failed")
		}
		srv.GracefulStop()
	}()

	if err := srv.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("serve")
	}
	log.Info().Msg("vh-agent stopped")
}
