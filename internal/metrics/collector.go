package metrics

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// memInfo holds parsed /proc/meminfo values in MB.
type memInfo struct {
	TotalMB uint64
	UsedMB  uint64
}

// parseMemInfo parses /proc/meminfo content.
// Used = Total - Available (Linux recommends MemAvailable for actual free memory).
func parseMemInfo(data []byte) (*memInfo, error) {
	var totalKB, availableKB uint64
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		rest = strings.TrimSuffix(rest, " kB")
		v, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "MemTotal":
			totalKB = v
		case "MemAvailable":
			availableKB = v
		}
	}
	if totalKB == 0 {
		return nil, fmt.Errorf("MemTotal not found in meminfo")
	}
	return &memInfo{
		TotalMB: totalKB / 1024,
		UsedMB:  (totalKB - availableKB) / 1024,
	}, nil
}

// cpuStat holds aggregate CPU times from /proc/stat.
type cpuStat struct {
	Total uint64
	Idle  uint64
}

// parseCPUStat parses the first "cpu " aggregate line from /proc/stat.
func parseCPUStat(data []byte) (cpuStat, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return cpuStat{}, fmt.Errorf("unexpected cpu line: %q", line)
		}
		var stat cpuStat
		for i := 1; i < 8; i++ {
			v, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				return cpuStat{}, fmt.Errorf("parse cpu field %d: %w", i, err)
			}
			stat.Total += v
			// fields[4] = idle, fields[5] = iowait → both count as "not working"
			if i == 4 || i == 5 {
				stat.Idle += v
			}
		}
		return stat, nil
	}
	return cpuStat{}, fmt.Errorf("cpu line not found in /proc/stat")
}

// cpuPctFromDelta returns CPU usage percentage from two consecutive snapshots.
func cpuPctFromDelta(prev, curr cpuStat) float64 {
	totalDelta := curr.Total - prev.Total
	idleDelta := curr.Idle - prev.Idle
	if totalDelta == 0 {
		return 0
	}
	nonIdle := totalDelta - idleDelta
	return float64(nonIdle) * 100.0 / float64(totalDelta)
}

// parseLoadAvg parses /proc/loadavg and returns the 1-minute average.
func parseLoadAvg(data []byte) (float64, error) {
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("empty loadavg")
	}
	return strconv.ParseFloat(fields[0], 64)
}

// wgStatus holds parsed fields from `wg show vhnet0` output.
type wgStatus struct {
	Endpoint        string
	HandshakeAgeSec int64 // -1 if never handshaked
}

// parseWGShow parses the output of `wg show vhnet0`.
// Extracts the first peer's endpoint and latest handshake age.
func parseWGShow(output string) (*wgStatus, error) {
	status := &wgStatus{HandshakeAgeSec: -1}
	lines := strings.Split(output, "\n")
	inPeer := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "peer:") {
			inPeer = true
			continue
		}
		if !inPeer {
			continue
		}
		if strings.HasPrefix(line, "endpoint:") {
			status.Endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
		}
		if strings.HasPrefix(line, "latest handshake:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
			if value == "(none)" || value == "" {
				status.HandshakeAgeSec = -1
				continue
			}
			// Parse "45 seconds ago" / "2 minutes, 30 seconds ago" / "1 hour ago"
			age, err := parseHandshakeAge(value)
			if err == nil {
				status.HandshakeAgeSec = age
			}
		}
	}
	return status, nil
}

// parseHandshakeAge converts "2 minutes, 30 seconds ago" → 150.
func parseHandshakeAge(s string) (int64, error) {
	s = strings.TrimSuffix(s, " ago")
	parts := strings.Split(s, ",")
	var total int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		fields := strings.Fields(p)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return 0, err
		}
		unit := strings.TrimSuffix(fields[1], "s")
		switch unit {
		case "second":
			total += n
		case "minute":
			total += n * 60
		case "hour":
			total += n * 3600
		case "day":
			total += n * 86400
		}
	}
	return total, nil
}

// Snapshot is the result of one metrics collection cycle.
type Snapshot struct {
	CPUPct            float64
	MemUsedMB         uint64
	MemTotalMB        uint64
	DiskUsedGB        float64
	DiskTotalGB       float64
	Load1m            float64
	WGHandshakeAgeSec int64
	WGEndpoint        string
}

// Collector reads system metrics. Paths are injectable for tests.
type Collector struct {
	StatPath    string
	MemInfoPath string
	LoadAvgPath string
	DiskPath    string
	WGRunner    func() (string, error) // returns output of `wg show vhnet0`

	prevCPU *cpuStat
}

// DefaultCollector returns a Collector wired to real /proc and wg show.
func DefaultCollector() *Collector {
	return &Collector{
		StatPath:    "/proc/stat",
		MemInfoPath: "/proc/meminfo",
		LoadAvgPath: "/proc/loadavg",
		DiskPath:    "/",
		WGRunner:    runWGShow,
	}
}

// Collect reads all metrics in one pass.
func (c *Collector) Collect() (*Snapshot, error) {
	snap := &Snapshot{}

	// CPU
	statBytes, err := os.ReadFile(c.StatPath)
	if err != nil {
		return nil, fmt.Errorf("read stat: %w", err)
	}
	curr, err := parseCPUStat(statBytes)
	if err != nil {
		return nil, fmt.Errorf("parse stat: %w", err)
	}
	if c.prevCPU != nil {
		snap.CPUPct = cpuPctFromDelta(*c.prevCPU, curr)
	}
	c.prevCPU = &curr

	// Memory
	memBytes, err := os.ReadFile(c.MemInfoPath)
	if err != nil {
		return nil, fmt.Errorf("read meminfo: %w", err)
	}
	mem, err := parseMemInfo(memBytes)
	if err != nil {
		return nil, fmt.Errorf("parse meminfo: %w", err)
	}
	snap.MemTotalMB = mem.TotalMB
	snap.MemUsedMB = mem.UsedMB

	// Loadavg
	loadBytes, err := os.ReadFile(c.LoadAvgPath)
	if err != nil {
		return nil, fmt.Errorf("read loadavg: %w", err)
	}
	load, err := parseLoadAvg(loadBytes)
	if err != nil {
		return nil, fmt.Errorf("parse loadavg: %w", err)
	}
	snap.Load1m = load

	// Disk
	var stat syscall.Statfs_t
	if err := syscall.Statfs(c.DiskPath, &stat); err != nil {
		return nil, fmt.Errorf("statfs %s: %w", c.DiskPath, err)
	}
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes
	const gb = 1024 * 1024 * 1024
	snap.DiskTotalGB = float64(totalBytes) / gb
	snap.DiskUsedGB = float64(usedBytes) / gb

	// WG
	if c.WGRunner != nil {
		out, err := c.WGRunner()
		if err == nil {
			wg, err := parseWGShow(out)
			if err == nil {
				snap.WGEndpoint = wg.Endpoint
				snap.WGHandshakeAgeSec = wg.HandshakeAgeSec
			}
		}
	}

	return snap, nil
}

// execCommand is a var so tests can override it.
var execCommand = func(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runWGShow executes `wg show vhnet0` and returns its stdout.
func runWGShow() (string, error) {
	out, err := execCommand("wg", "show", "vhnet0")
	if err != nil {
		return "", fmt.Errorf("wg show: %w (output: %s)", err, out)
	}
	return out, nil
}
