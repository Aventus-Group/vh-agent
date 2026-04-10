// Package server wires the individual sub-services into a single grpc.Server.
package server

import (
	"context"
	"time"

	"github.com/Aventus-Group/vh-agent/gen/agentpb"
	"github.com/Aventus-Group/vh-agent/internal/execsvc"
	"github.com/Aventus-Group/vh-agent/internal/fssvc"
	"github.com/Aventus-Group/vh-agent/internal/healthsvc"
	"github.com/Aventus-Group/vh-agent/internal/jobs"
	"google.golang.org/grpc"
)

// Facade implements agentpb.VibhostAgentServer by delegating to sub-services.
type Facade struct {
	agentpb.UnimplementedVibhostAgentServer
	exec   *execsvc.Service
	fs     *fssvc.Service
	health *healthsvc.Service
}

// NewFacade constructs a facade ready to be registered on a gRPC server.
func NewFacade(version, defaultWorkdir string) *Facade {
	return &Facade{
		exec:   execsvc.New(jobs.New(), defaultWorkdir),
		fs:     fssvc.New(),
		health: healthsvc.New(version, time.Now()),
	}
}

// Exec delegates to the exec sub-service.
func (f *Facade) Exec(req *agentpb.ExecRequest, stream agentpb.VibhostAgent_ExecServer) error {
	return f.exec.Exec(req, stream)
}

// Kill delegates to the exec sub-service.
func (f *Facade) Kill(ctx context.Context, req *agentpb.KillRequest) (*agentpb.KillResponse, error) {
	return f.exec.Kill(ctx, req)
}

// ReadFile delegates to the fs sub-service.
func (f *Facade) ReadFile(ctx context.Context, req *agentpb.ReadFileRequest) (*agentpb.ReadFileResponse, error) {
	return f.fs.ReadFile(ctx, req)
}

// WriteFile delegates to the fs sub-service.
func (f *Facade) WriteFile(ctx context.Context, req *agentpb.WriteFileRequest) (*agentpb.WriteFileResponse, error) {
	return f.fs.WriteFile(ctx, req)
}

// ListDir delegates to the fs sub-service.
func (f *Facade) ListDir(ctx context.Context, req *agentpb.ListDirRequest) (*agentpb.ListDirResponse, error) {
	return f.fs.ListDir(ctx, req)
}

// Health delegates to the health sub-service.
func (f *Facade) Health(ctx context.Context, req *agentpb.HealthRequest) (*agentpb.HealthResponse, error) {
	return f.health.Health(ctx, req)
}

// New returns a configured grpc.Server with the facade registered and the
// standard interceptor chain installed.
func New(version, defaultWorkdir string) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(UnaryLoggingInterceptor, UnaryRecoveryInterceptor),
		grpc.ChainStreamInterceptor(StreamLoggingInterceptor, StreamRecoveryInterceptor),
	)
	agentpb.RegisterVibhostAgentServer(srv, NewFacade(version, defaultWorkdir))
	return srv
}
