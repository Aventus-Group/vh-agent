package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMemInfo(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "proc_meminfo"))
	if err != nil {
		t.Fatal(err)
	}
	mem, err := parseMemInfo(data)
	if err != nil {
		t.Fatalf("parseMemInfo: %v", err)
	}
	if mem.TotalMB != 1024 {
		t.Errorf("TotalMB = %d, want 1024", mem.TotalMB)
	}
	if mem.UsedMB != 512 {
		t.Errorf("UsedMB = %d, want 512", mem.UsedMB)
	}
}

func TestParseCPUStat(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "proc_stat"))
	if err != nil {
		t.Fatal(err)
	}
	stat, err := parseCPUStat(data)
	if err != nil {
		t.Fatalf("parseCPUStat: %v", err)
	}
	// 1000+50+500+8000+100+0+50 = 9700
	if stat.Total != 9700 {
		t.Errorf("Total = %d, want 9700", stat.Total)
	}
	// idle+iowait = 8000+100 = 8100
	if stat.Idle != 8100 {
		t.Errorf("Idle = %d, want 8100", stat.Idle)
	}
}

func TestCPUPctFromDelta(t *testing.T) {
	prev := cpuStat{Total: 10000, Idle: 9000}
	curr := cpuStat{Total: 11000, Idle: 9800}
	// delta total = 1000, delta idle = 800 → non-idle = 200 → 20%
	pct := cpuPctFromDelta(prev, curr)
	if pct < 19.9 || pct > 20.1 {
		t.Errorf("pct = %f, want ~20", pct)
	}
}

func TestCPUPctFromDelta_NoChange(t *testing.T) {
	prev := cpuStat{Total: 10000, Idle: 9000}
	curr := cpuStat{Total: 10000, Idle: 9000}
	pct := cpuPctFromDelta(prev, curr)
	if pct != 0 {
		t.Errorf("pct = %f, want 0", pct)
	}
}
