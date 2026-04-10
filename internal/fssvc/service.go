// Package fssvc implements the file-related gRPC handlers of VibhostAgent.
package fssvc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Aventus-Group/vh-agent/gen/agentpb"
)

const defaultReadLimit = 1 * 1024 * 1024 // 1 MiB

// Service is the FileService implementation. It is embedded into the
// composite server alongside exec and health services.
type Service struct {
	agentpb.UnimplementedVibhostAgentServer
}

// New returns a ready-to-use file service.
func New() *Service { return &Service{} }

// ReadFile reads a file up to max_bytes. Size always reflects the real file
// size; truncated indicates whether the returned content is partial.
func (s *Service) ReadFile(_ context.Context, req *agentpb.ReadFileRequest) (*agentpb.ReadFileResponse, error) {
	info, err := os.Stat(req.Path)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("path is a directory")
	}

	limit := req.MaxBytes
	if limit <= 0 {
		limit = defaultReadLimit
	}

	f, err := os.Open(req.Path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	content := make([]byte, limit)
	n, err := io.ReadFull(f, content)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read: %w", err)
	}
	content = content[:n]
	return &agentpb.ReadFileResponse{
		Content:   content,
		Size:      info.Size(),
		Truncated: info.Size() > int64(n),
	}, nil
}

// WriteFile writes content to path, optionally creating parent directories.
func (s *Service) WriteFile(_ context.Context, req *agentpb.WriteFileRequest) (*agentpb.WriteFileResponse, error) {
	if req.CreateDirs {
		if err := os.MkdirAll(filepath.Dir(req.Path), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir: %w", err)
		}
	}
	mode := os.FileMode(req.Mode)
	if mode == 0 {
		mode = 0o644
	}
	if err := os.WriteFile(req.Path, req.Content, mode); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	return &agentpb.WriteFileResponse{BytesWritten: int64(len(req.Content))}, nil
}

// ListDir returns one-level directory entries.
func (s *Service) ListDir(_ context.Context, req *agentpb.ListDirRequest) (*agentpb.ListDirResponse, error) {
	entries, err := os.ReadDir(req.Path)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	out := make([]*agentpb.DirEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, &agentpb.DirEntry{
			Name:  e.Name(),
			Size:  info.Size(),
			IsDir: e.IsDir(),
			Mode:  int32(info.Mode().Perm()),
		})
	}
	return &agentpb.ListDirResponse{Entries: out}, nil
}
