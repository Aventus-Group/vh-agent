package execsvc

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Aventus-Group/vh-agent/gen/agentpb"
	"github.com/Aventus-Group/vh-agent/internal/jobs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func newTestServer(t *testing.T) agentpb.VibhostAgentClient {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	agentpb.RegisterVibhostAgentServer(srv, New(jobs.New(), "/tmp"))
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		srv.Stop()
		lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(context.Background())
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return agentpb.NewVibhostAgentClient(conn)
}

func collectExec(t *testing.T, stream agentpb.VibhostAgent_ExecClient) ([]string, []string, *agentpb.ExitedEvent, int32) {
	t.Helper()
	var (
		stdout []string
		stderr []string
		exited *agentpb.ExitedEvent
		pid    int32
	)
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		switch e := ev.Event.(type) {
		case *agentpb.ExecEvent_Started:
			pid = e.Started.Pid
		case *agentpb.ExecEvent_StdoutChunk:
			stdout = append(stdout, string(e.StdoutChunk))
		case *agentpb.ExecEvent_StderrChunk:
			stderr = append(stderr, string(e.StderrChunk))
		case *agentpb.ExecEvent_Exited:
			exited = e.Exited
		}
	}
	return stdout, stderr, exited, pid
}

func TestExec_EchoStdout(t *testing.T) {
	client := newTestServer(t)
	stream, err := client.Exec(context.Background(), &agentpb.ExecRequest{
		Cmd:   "echo hello",
		JobId: "echo-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, exited, pid := collectExec(t, stream)
	if pid == 0 {
		t.Error("expected non-zero pid in Started event")
	}
	if exited == nil || exited.ExitCode != 0 {
		t.Errorf("expected clean exit, got %#v", exited)
	}
	joined := strings.Join(stdout, "")
	if !strings.Contains(joined, "hello") {
		t.Errorf("stdout = %q want contains 'hello'", joined)
	}
	if len(stderr) != 0 {
		t.Errorf("unexpected stderr: %q", stderr)
	}
}

func TestExec_ExitCodeNonZero(t *testing.T) {
	client := newTestServer(t)
	stream, err := client.Exec(context.Background(), &agentpb.ExecRequest{
		Cmd:   "exit 42",
		JobId: "exit-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, exited, _ := collectExec(t, stream)
	if exited == nil {
		t.Fatal("no Exited event")
	}
	if exited.ExitCode != 42 {
		t.Errorf("exit_code = %d want 42", exited.ExitCode)
	}
}

func TestExec_StderrIsRouted(t *testing.T) {
	client := newTestServer(t)
	stream, err := client.Exec(context.Background(), &agentpb.ExecRequest{
		Cmd:   "echo oops 1>&2",
		JobId: "stderr-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, exited, _ := collectExec(t, stream)
	if exited == nil || exited.ExitCode != 0 {
		t.Errorf("unexpected exit: %#v", exited)
	}
	if len(stdout) != 0 {
		t.Errorf("unexpected stdout: %q", stdout)
	}
	if !strings.Contains(strings.Join(stderr, ""), "oops") {
		t.Errorf("stderr = %q want contains 'oops'", stderr)
	}
}

func TestExec_WorkdirDefault(t *testing.T) {
	client := newTestServer(t)
	stream, err := client.Exec(context.Background(), &agentpb.ExecRequest{
		Cmd:   "pwd",
		JobId: "pwd-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, _, _ := collectExec(t, stream)
	if !strings.Contains(strings.Join(stdout, ""), "/tmp") {
		t.Errorf("pwd output = %q want /tmp (defaultWorkdir)", stdout)
	}
}

func TestExec_CustomWorkdir(t *testing.T) {
	dir := t.TempDir()
	client := newTestServer(t)
	stream, err := client.Exec(context.Background(), &agentpb.ExecRequest{
		Cmd:     "pwd",
		Workdir: dir,
		JobId:   "pwd-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, _, _ := collectExec(t, stream)
	if !strings.Contains(strings.Join(stdout, ""), dir) {
		t.Errorf("pwd = %q want %s", stdout, dir)
	}
}

func TestExec_EnvPassthrough(t *testing.T) {
	client := newTestServer(t)
	stream, err := client.Exec(context.Background(), &agentpb.ExecRequest{
		Cmd:   "echo $FOO_TEST_VAR",
		Env:   map[string]string{"FOO_TEST_VAR": "bar-value"},
		JobId: "env-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, _, _ := collectExec(t, stream)
	if !strings.Contains(strings.Join(stdout, ""), "bar-value") {
		t.Errorf("stdout = %q want 'bar-value'", stdout)
	}
}

func TestExec_Timeout(t *testing.T) {
	client := newTestServer(t)
	stream, err := client.Exec(context.Background(), &agentpb.ExecRequest{
		Cmd:        "sleep 5",
		TimeoutSec: 1,
		JobId:      "timeout-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, _, exited, _ := collectExec(t, stream)
	elapsed := time.Since(start)

	if exited == nil || !exited.TimedOut {
		t.Errorf("expected timed_out=true, got %#v", exited)
	}
	if elapsed > 4*time.Second {
		t.Errorf("took too long: %v", elapsed)
	}
}

func TestExec_KillByJobID(t *testing.T) {
	client := newTestServer(t)
	stream, err := client.Exec(context.Background(), &agentpb.ExecRequest{
		Cmd:   "sleep 30",
		JobId: "kill-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Wait for Started event before killing.
	firstEv, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if firstEv.GetStarted() == nil {
		t.Fatalf("first event not Started: %#v", firstEv)
	}

	// Kill via Kill RPC.
	resp, err := client.Kill(context.Background(), &agentpb.KillRequest{JobId: "kill-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Killed {
		t.Fatal("expected killed=true")
	}

	// Drain remaining events.
	_, _, exited, _ := collectExec(t, stream)
	if exited == nil {
		t.Fatal("no Exited event after Kill")
	}
}

func TestKill_UnknownJobID(t *testing.T) {
	client := newTestServer(t)
	resp, err := client.Kill(context.Background(), &agentpb.KillRequest{JobId: "does-not-exist"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Killed {
		t.Error("expected killed=false for unknown job_id")
	}
}
