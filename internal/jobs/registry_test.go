package jobs

import (
	"os/exec"
	"testing"
)

func TestRegistry_RegisterAndFind(t *testing.T) {
	r := New()
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	r.Register("job-1", cmd.Process)

	proc, ok := r.Lookup("job-1")
	if !ok {
		t.Fatal("Lookup(job-1): not found")
	}
	if proc.Pid != cmd.Process.Pid {
		t.Errorf("pid mismatch: got %d want %d", proc.Pid, cmd.Process.Pid)
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := New()
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	r.Register("job-2", cmd.Process)
	r.Unregister("job-2")

	if _, ok := r.Lookup("job-2"); ok {
		t.Fatal("Lookup(job-2): still present after Unregister")
	}
}

func TestRegistry_LookupMissing(t *testing.T) {
	r := New()
	if _, ok := r.Lookup("ghost"); ok {
		t.Fatal("Lookup(ghost): unexpected hit")
	}
}

func TestRegistry_OverwriteIsNoOp(t *testing.T) {
	// Registering the same job_id twice should keep the first process —
	// clients must not reuse job_id. Second Register is silently ignored.
	r := New()
	first := exec.Command("sleep", "10")
	second := exec.Command("sleep", "10")
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	defer first.Process.Kill()
	defer second.Process.Kill()

	r.Register("dup", first.Process)
	r.Register("dup", second.Process)

	got, _ := r.Lookup("dup")
	if got.Pid != first.Process.Pid {
		t.Errorf("duplicate Register overwrote first; got pid %d want %d", got.Pid, first.Process.Pid)
	}
}
