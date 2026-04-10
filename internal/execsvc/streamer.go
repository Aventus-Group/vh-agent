// Package execsvc implements the gRPC ExecService. This file contains the
// StreamChunks helper that reads from an io.Reader and forwards chunks to
// a send callback on a size-or-time boundary.
package execsvc

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"
)

// StreamOptions controls how bytes are batched into chunks.
type StreamOptions struct {
	// ChunkSize is the maximum bytes per chunk. Default 4096.
	ChunkSize int
	// FlushInterval is the maximum time buffered bytes may sit before
	// being flushed. Default 100ms.
	FlushInterval time.Duration
}

func (o *StreamOptions) defaults() {
	if o.ChunkSize <= 0 {
		o.ChunkSize = 4096
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = 100 * time.Millisecond
	}
}

// SendFunc receives a chunk. The slice is safe to retain until the function
// returns — the caller guarantees no reuse.
type SendFunc func(chunk []byte) error

// StreamChunks reads from r and invokes send for every chunk. It stops when
// r reaches EOF, the context is cancelled, or send returns an error. The
// chunk passed to send is a fresh slice, so the caller may keep references.
func StreamChunks(ctx context.Context, r io.Reader, send SendFunc, opts StreamOptions) error {
	opts.defaults()

	buf := make([]byte, opts.ChunkSize)
	pending := &bytes.Buffer{}
	pending.Grow(opts.ChunkSize)

	// Reader goroutine sends read results on chan; select multiplexes with
	// ticker and context for deterministic flushing.
	//
	// data carries a snapshot of the bytes read (not a slice into buf) so
	// the goroutine and main loop never race on the underlying array.
	type readResult struct {
		data []byte
		err  error
	}
	reads := make(chan readResult, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			n, err := r.Read(buf)
			var snapshot []byte
			if n > 0 {
				snapshot = make([]byte, n)
				copy(snapshot, buf[:n])
			}
			reads <- readResult{data: snapshot, err: err}
			if err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(opts.FlushInterval)
	defer ticker.Stop()

	flush := func() error {
		if pending.Len() == 0 {
			return nil
		}
		chunk := make([]byte, pending.Len())
		copy(chunk, pending.Bytes())
		pending.Reset()
		return send(chunk)
	}

	for {
		select {
		case <-ctx.Done():
			_ = flush()
			return ctx.Err()
		case res := <-reads:
			if len(res.data) > 0 {
				// Write the snapshot into pending, then emit chunks of
				// exactly ChunkSize while pending has enough data.
				pending.Write(res.data)
				for pending.Len() >= opts.ChunkSize {
					chunk := make([]byte, opts.ChunkSize)
					copy(chunk, pending.Next(opts.ChunkSize))
					if err := send(chunk); err != nil {
						return err
					}
				}
			}
			if res.err != nil {
				if err := flush(); err != nil {
					return err
				}
				if res.err == io.EOF {
					return nil
				}
				return res.err
			}
		case <-ticker.C:
			if err := flush(); err != nil {
				return err
			}
		}
	}
}
