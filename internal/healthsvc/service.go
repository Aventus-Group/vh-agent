// Package healthsvc implements the Health gRPC handler.
package healthsvc

import (
	"context"
	"time"

	"github.com/Aventus-Group/vh-agent/gen/agentpb"
)

// Service responds to Health probes.
type Service struct {
	agentpb.UnimplementedVibhostAgentServer
	version   string
	startedAt time.Time
}

// New creates a health service with a fixed version and start time.
func New(version string, startedAt time.Time) *Service {
	return &Service{version: version, startedAt: startedAt}
}

// Health returns the configured version and computed uptime.
func (s *Service) Health(_ context.Context, _ *agentpb.HealthRequest) (*agentpb.HealthResponse, error) {
	return &agentpb.HealthResponse{
		Version:   s.version,
		UptimeSec: int64(time.Since(s.startedAt).Seconds()),
	}, nil
}
