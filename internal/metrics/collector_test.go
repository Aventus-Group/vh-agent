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

func TestParseLoadAvg(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "proc_loadavg"))
	if err != nil {
		t.Fatal(err)
	}
	load, err := parseLoadAvg(data)
	if err != nil {
		t.Fatalf("parseLoadAvg: %v", err)
	}
	if load < 0.14 || load > 0.16 {
		t.Errorf("load = %f, want ~0.15", load)
	}
}

func TestParseWGShow(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "wg_show_output"))
	if err != nil {
		t.Fatal(err)
	}
	wg := parseWGShow(string(data))
	if wg.Endpoint != "78.47.77.236:48720" {
		t.Errorf("Endpoint = %q", wg.Endpoint)
	}
	if wg.HandshakeAgeSec != 45 {
		t.Errorf("HandshakeAgeSec = %d, want 45", wg.HandshakeAgeSec)
	}
}

func TestParseWGShow_NeverHandshaked(t *testing.T) {
	input := `interface: vhnet0
peer: abc=
  endpoint: 1.2.3.4:48720
  latest handshake: (none)
`
	wg := parseWGShow(input)
	if wg.HandshakeAgeSec != -1 {
		t.Errorf("HandshakeAgeSec = %d, want -1 (sentinel for never)", wg.HandshakeAgeSec)
	}
}

func TestCollector_Collect(t *testing.T) {
	// Use testdata fixtures — Collector accepts injectable file readers for testing.
	c := &Collector{
		StatPath:    filepath.Join("testdata", "proc_stat"),
		MemInfoPath: filepath.Join("testdata", "proc_meminfo"),
		LoadAvgPath: filepath.Join("testdata", "proc_loadavg"),
		DiskPath:    "/", // real root — just verifies Statfs works without error
		WGRunner: func() (string, error) {
			data, err := os.ReadFile(filepath.Join("testdata", "wg_show_output"))
			return string(data), err
		},
	}

	// First Collect: CPU snapshot is stored, CPU% is 0 (no prior sample).
	snap1, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if snap1.MemTotalMB != 1024 {
		t.Errorf("MemTotalMB = %d", snap1.MemTotalMB)
	}
	if snap1.MemUsedMB != 512 {
		t.Errorf("MemUsedMB = %d", snap1.MemUsedMB)
	}
	if snap1.WGEndpoint != "78.47.77.236:48720" {
		t.Errorf("WGEndpoint = %q", snap1.WGEndpoint)
	}
	if snap1.DiskTotalGB <= 0 {
		t.Errorf("DiskTotalGB = %f, want > 0", snap1.DiskTotalGB)
	}
}
