// Package jobs provides a concurrency-safe mapping from client-assigned
// job identifiers to the underlying os.Process, so the Exec service can
// honour Kill requests without races.
package jobs

import (
	"os"
	"sync"
)

// Registry holds running jobs indexed by job_id.
type Registry struct {
	mu   sync.Mutex
	jobs map[string]*os.Process
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{jobs: make(map[string]*os.Process)}
}

// Register stores proc under jobID. Duplicate registrations are ignored
// (first writer wins) to keep Kill semantics predictable.
func (r *Registry) Register(jobID string, proc *os.Process) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.jobs[jobID]; exists {
		return
	}
	r.jobs[jobID] = proc
}

// Unregister removes jobID from the registry. No-op if absent.
func (r *Registry) Unregister(jobID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.jobs, jobID)
}

// Lookup returns the process registered under jobID, or (nil, false) if
// absent.
func (r *Registry) Lookup(jobID string) (*os.Process, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	proc, ok := r.jobs[jobID]
	return proc, ok
}
