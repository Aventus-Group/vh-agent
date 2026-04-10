package execsvc

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamer_ReadsWholeStream(t *testing.T) {
	src := strings.NewReader("hello world")
	ctx := context.Background()
	var (
		mu     sync.Mutex
		chunks [][]byte
	)
	send := func(chunk []byte) error {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]byte, len(chunk))
		copy(cp, chunk)
		chunks = append(chunks, cp)
		return nil
	}

	if err := StreamChunks(ctx, src, send, StreamOptions{ChunkSize: 1024, FlushInterval: 50 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	joined := bytes.Join(chunks, nil)
	if string(joined) != "hello world" {
		t.Errorf("joined chunks = %q want %q", joined, "hello world")
	}
}

func TestStreamer_SplitsAtChunkSize(t *testing.T) {
	// 10-byte reader with ChunkSize=4 → expect chunks of 4, 4, 2.
	src := strings.NewReader("0123456789")
	ctx := context.Background()
	var (
		mu     sync.Mutex
		chunks [][]byte
	)
	send := func(chunk []byte) error {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]byte, len(chunk))
		copy(cp, chunk)
		chunks = append(chunks, cp)
		return nil
	}

	if err := StreamChunks(ctx, src, send, StreamOptions{ChunkSize: 4, FlushInterval: time.Second}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(chunks) != 3 {
		t.Fatalf("chunk count = %d want 3", len(chunks))
	}
	if string(chunks[0]) != "0123" || string(chunks[1]) != "4567" || string(chunks[2]) != "89" {
		t.Errorf("chunks = %q", chunks)
	}
}

func TestStreamer_StopsOnContextCancel(t *testing.T) {
	// Block forever until ctx cancel, then ensure StreamChunks returns.
	pr, pw := blockingPipe(t)
	defer pr.Close()
	defer pw.Close()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- StreamChunks(ctx, pr, func(_ []byte) error { return nil }, StreamOptions{ChunkSize: 32, FlushInterval: 10 * time.Millisecond})
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamChunks did not return after cancel")
	}
}

// blockingPipe returns an io.PipeReader/Writer pair where the reader
// blocks forever until either the writer writes or the test closes it.
func blockingPipe(t *testing.T) (*PipeReader, *PipeWriter) {
	t.Helper()
	return NewPipe()
}
