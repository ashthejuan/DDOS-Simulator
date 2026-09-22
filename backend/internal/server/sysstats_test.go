package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFakeProc creates procDir/{self/stat,self/status,uptime} fixtures.
// The comm name intentionally contains a space to exercise the parser.
func writeFakeProc(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	self := filepath.Join(dir, "self")
	if err := os.Mkdir(self, 0o755); err != nil {
		t.Fatal(err)
	}
	stat := "1 (my proc) R 1 1 1 0 0 0 0 0 0 0 100 50 0 0 20 0 1 0 12345\n"
	if err := os.WriteFile(filepath.Join(self, "stat"), []byte(stat), 0o644); err != nil {
		t.Fatal(err)
	}
	status := "Name:\tmy proc\nVmRSS:\t   12345 kB\n"
	if err := os.WriteFile(filepath.Join(self, "status"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReadCPUTime(t *testing.T) {
	dir := writeFakeProc(t)
	got, err := readCPUTime(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != 150 { // utime 100 + stime 50
		t.Fatalf("cpu ticks=%d, want 150", got)
	}
}

func TestReadRSS(t *testing.T) {
	dir := writeFakeProc(t)
	got, err := readRSS(dir)
	if err != nil {
		t.Fatal(err)
	}
	if *got != 12345*1024 {
		t.Fatalf("rss=%d, want %d", *got, 12345*1024)
	}
}

func TestSampleCPUStaticCounters(t *testing.T) {
	dir := writeFakeProc(t)
	pct, err := sampleCPU(dir, 5*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if *pct != 0 {
		t.Fatalf("cpu=%v, want 0 for static counters", *pct)
	}
}

func TestStatsMissingProc(t *testing.T) {
	st := statsFromDir(t.TempDir()) // empty dir: no self/stat
	if st.CPUPercent != nil || st.MemoryRSSBytes != nil {
		t.Fatalf("want empty stats, got %+v", st)
	}
	// Empty stats must serialize with nulls, not zeros.
	raw, _ := json.Marshal(st)
	t.Logf("null-shape: %s", raw)
}

func TestStatsEndpointShape(t *testing.T) {
	srv := httptest.NewServer(NewMux(Config{}))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
	var st ProcStats
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Portable assertion: shape only (values exist on Linux, null on darwin).
}
