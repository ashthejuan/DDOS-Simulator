// Crude test-process telemetry (Phase 3, Linux-only).
//
// Reads /proc/self/stat, /proc/uptime and /proc/self/status — zero
// dependencies. On non-Linux systems (e.g. macOS local runs) the files
// are absent and stats come back empty (JSON nulls), which is fine:
// Docker is the canonical demo path where /proc exists.
package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// clockTick is USER_HZ on Linux (100 on x86_64 and arm64).
const clockTick = 100

// cpuSampleInterval is the gap between the two CPU counter reads.
const cpuSampleInterval = 100 * time.Millisecond

// ProcStats are nullable so "unavailable" serializes as JSON null.
type ProcStats struct {
	// CPUPercent is recent process CPU as % of one core (top-style,
	// can exceed 100 with multiple threads). Nil when unavailable.
	CPUPercent *float64 `json:"cpu_percent"`
	// MemoryRSSBytes is resident set size. Nil when unavailable.
	MemoryRSSBytes *int64 `json:"memory_rss_bytes"`
}

// CurrentStats samples this process: two CPU counter reads cpuSampleInterval
// apart plus current RSS. Never fails — missing /proc yields empty stats.
func CurrentStats() ProcStats {
	return statsFromDir("/proc")
}

func statsFromDir(procDir string) ProcStats {
	var st ProcStats
	if pct, err := sampleCPU(procDir, cpuSampleInterval); err == nil {
		st.CPUPercent = pct
	}
	if rss, err := readRSS(procDir); err == nil {
		st.MemoryRSSBytes = rss
	}
	return st
}

// sampleCPU returns process CPU % over interval via utime+stime deltas.
func sampleCPU(procDir string, interval time.Duration) (*float64, error) {
	before, err := readCPUTime(procDir)
	if err != nil {
		return nil, err
	}
	time.Sleep(interval)
	after, err := readCPUTime(procDir)
	if err != nil {
		return nil, err
	}
	pct := float64(after-before) / clockTick / interval.Seconds() * 100
	return &pct, nil
}

// readCPUTime returns utime+stime in clock ticks for this process.
func readCPUTime(procDir string) (uint64, error) {
	raw, err := os.ReadFile(filepath.Join(procDir, "self", "stat"))
	if err != nil {
		return 0, err
	}
	// comm (field 2) may contain spaces/parens: split after the last ')'.
	s := string(raw)
	i := strings.LastIndex(s, ")")
	if i < 0 {
		return 0, fmt.Errorf("malformed stat")
	}
	fields := strings.Fields(s[i+1:])
	// fields[0] is now field 3 (state); utime=14 -> index 11, stime=15 -> 12.
	if len(fields) < 13 {
		return 0, fmt.Errorf("malformed stat")
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, err
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, err
	}
	return utime + stime, nil
}

// readRSS returns resident set size in bytes from VmRSS.
func readRSS(procDir string) (*int64, error) {
	raw, err := os.ReadFile(filepath.Join(procDir, "self", "status"))
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(line, "VmRSS:"); ok {
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				return nil, fmt.Errorf("malformed VmRSS")
			}
			kb, err := strconv.ParseInt(fields[0], 10, 64)
			if err != nil {
				return nil, err
			}
			b := kb * 1024
			return &b, nil
		}
	}
	return nil, fmt.Errorf("VmRSS not found")
}
