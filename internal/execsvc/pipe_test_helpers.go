// Helpers shared by _test.go files. Kept outside the generated code.

package execsvc

import "io"

type (
	PipeReader = io.PipeReader
	PipeWriter = io.PipeWriter
)

// NewPipe returns a fresh in-memory pipe.
func NewPipe() (*PipeReader, *PipeWriter) {
	return io.Pipe()
}
