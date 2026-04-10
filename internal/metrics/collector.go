package metrics

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
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
