package execsvc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/Aventus-Group/vh-agent/gen/agentpb"
	"github.com/Aventus-Group/vh-agent/internal/jobs"
)

// Service implements agentpb.VibhostAgentServer for exec/kill. File and
// health services are implemented elsewhere; a composite server wires them
// all together.
type Service struct {
	agentpb.UnimplementedVibhostAgentServer
	registry       *jobs.Registry
	defaultWorkdir string
}

// New returns an exec service using the supplied registry. defaultWorkdir is
// used when an ExecRequest omits the workdir field.
func New(registry *jobs.Registry, defaultWorkdir string) *Service {
	return &Service{registry: registry, defaultWorkdir: defaultWorkdir}
}

// Exec runs a shell command and streams stdout/stderr to the client.
func (s *Service) Exec(req *agentpb.ExecRequest, stream agentpb.VibhostAgent_ExecServer) error {
	if req.Cmd == "" {
		return sendExitError(stream, "cmd is required")
	}
	workdir := req.Workdir
	if workdir == "" {
		workdir = s.defaultWorkdir
	}

	ctx := stream.Context()
	var cancel context.CancelFunc
	serverTimeout := time.Duration(0)
	if req.TimeoutSec > 0 {
		serverTimeout = time.Duration(req.TimeoutSec) * time.Second
		ctx, cancel = context.WithTimeout(ctx, serverTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", req.Cmd)
	cmd.Dir = workdir
	cmd.Env = buildEnv(req.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return sendExitError(stream, "stdout pipe: "+err.Error())
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return sendExitError(stream, "stderr pipe: "+err.Error())
	}

	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		return sendExitError(stream, "start: "+err.Error())
	}

	jobID := req.JobId
	if jobID != "" {
		s.registry.Register(jobID, cmd.Process)
		defer s.registry.Unregister(jobID)
	}

	if err := stream.Send(&agentpb.ExecEvent{
		Event: &agentpb.ExecEvent_Started{
			Started: &agentpb.StartedEvent{
				Pid:             int32(cmd.Process.Pid),
				StartedAtUnixMs: startedAt.UnixMilli(),
			},
		},
	}); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}

	var sendMu sync.Mutex
	safeSend := func(ev *agentpb.ExecEvent) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(ev)
	}

	var wg sync.WaitGroup
	streamErrCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := StreamChunks(ctx, stdoutPipe, func(chunk []byte) error {
			return safeSend(&agentpb.ExecEvent{Event: &agentpb.ExecEvent_StdoutChunk{StdoutChunk: chunk}})
		}, StreamOptions{})
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			streamErrCh <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := StreamChunks(ctx, stderrPipe, func(chunk []byte) error {
			return safeSend(&agentpb.ExecEvent{Event: &agentpb.ExecEvent_StderrChunk{StderrChunk: chunk}})
		}, StreamOptions{})
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			streamErrCh <- err
		}
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	close(streamErrCh)

	exitCode := int32(0)
	errorMessage := ""
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = int32(exitErr.ExitCode())
		} else {
			exitCode = -1
			errorMessage = waitErr.Error()
		}
	}

	timedOut := false
	if serverTimeout > 0 {
		if err := ctx.Err(); errors.Is(err, context.DeadlineExceeded) {
			timedOut = true
		}
	}

	for streamErr := range streamErrCh {
		if errorMessage == "" {
			errorMessage = streamErr.Error()
		}
	}

	duration := time.Since(startedAt).Milliseconds()
	return safeSend(&agentpb.ExecEvent{
		Event: &agentpb.ExecEvent_Exited{
			Exited: &agentpb.ExitedEvent{
				ExitCode:     exitCode,
				DurationMs:   duration,
				TimedOut:     timedOut,
				ErrorMessage: errorMessage,
			},
		},
	})
}

// Kill signals a running process registered under jobID.
func (s *Service) Kill(_ context.Context, req *agentpb.KillRequest) (*agentpb.KillResponse, error) {
	proc, ok := s.registry.Lookup(req.JobId)
	if !ok {
		return &agentpb.KillResponse{Killed: false}, nil
	}
	sig := syscall.SIGTERM
	if req.Signal == int32(syscall.SIGKILL) {
		sig = syscall.SIGKILL
	}
	if err := proc.Signal(sig); err != nil {
		return nil, fmt.Errorf("signal process: %w", err)
	}
	return &agentpb.KillResponse{Killed: true}, nil
}

// sendExitError is a shortcut used when the request is invalid and we want
// to surface the failure as an ExitedEvent rather than a gRPC error.
func sendExitError(stream agentpb.VibhostAgent_ExecServer, msg string) error {
	return stream.Send(&agentpb.ExecEvent{
		Event: &agentpb.ExecEvent_Exited{
			Exited: &agentpb.ExitedEvent{
				ExitCode:     -1,
				ErrorMessage: msg,
			},
		},
	})
}

// buildEnv returns os.Environ() plus overrides, giving the request explicit
// keys precedence.
func buildEnv(extras map[string]string) []string {
	env := append([]string{}, os.Environ()...)
	for k, v := range extras {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
