package report

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"
)

func testInput() Input {
	start := time.Now().UTC().Add(-30 * time.Second)
	end := time.Now().UTC()
	return Input{
		ID:                 "abc123",
		Status:             "completed",
		StartedAt:          start,
		CompletedAt:        &end,
		TargetURL:          "http://localhost:8081/api/test",
		Defense:            true,
		DurationSeconds:    30,
		RequestsPerSecond:  100,
		Workers:            10,
		TotalRequests:      3000,
		SuccessfulRequests: 2700,
		FailedRequests:     300,
		RPS:                95.5,
		AvgLatencyMs:       12.3,
		P50LatencyMs:       8.1,
		P95LatencyMs:       41.2,
		P99LatencyMs:       88.0,
		Samples:            3000,
		StatusCodes:        map[string]int64{"200": 2700, "429": 300},
	}
}

func TestBuildProducesPDF(t *testing.T) {
	pdf, err := Build(testInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(pdf) == 0 {
		t.Fatal("empty PDF")
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("output does not start with %PDF")
	}
}

// TestBuildEncodesLatin1 ensures typographic chars (· — •) are written as
// cp1252 single bytes for the WinAnsi core fonts, not raw UTF-8 (which
// viewers render as mojibake like Â· / â€" / â€¢).
func TestBuildEncodesLatin1(t *testing.T) {
	pdf, err := Build(testInput())
	if err != nil {
		t.Fatal(err)
	}
	streamRe := regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)
	var text bytes.Buffer
	for _, m := range streamRe.FindAllSubmatch(pdf, -1) {
		zr, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // metadata etc.; only page streams matter
		}
		raw, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			t.Fatal(err)
		}
		text.Write(raw)
	}
	body := text.Bytes()
	for _, seq := range [][]byte{
		{0xC2, 0xB7},       // · as UTF-8
		{0xE2, 0x80, 0x94}, // — as UTF-8
		{0xE2, 0x80, 0xA2}, // • as UTF-8
	} {
		if bytes.Contains(body, seq) {
			t.Fatalf("page stream contains raw UTF-8 %x (mojibake in viewers)", seq)
		}
	}
	for _, b := range []byte{0xB7, 0x97, 0x95} { // · — • as cp1252
		if !bytes.Contains(body, []byte{b}) {
			t.Fatalf("page stream missing cp1252 byte %x", b)
		}
	}
}

func TestObserveDefenseEngaged(t *testing.T) {
	obs := Observe(testInput())
	joined := strings.Join(obs, "\n")
	if !strings.Contains(joined, "429") {
		t.Fatalf("want 429 observation, got:\n%s", joined)
	}
	if !strings.Contains(joined, "Tail latency") {
		t.Fatalf("want tail-latency observation (P95 %.1f >> P50 %.1f), got:\n%s", 41.2, 8.1, joined)
	}
}

func TestObserveBaselineClean(t *testing.T) {
	in := testInput()
	in.Defense = false
	in.FailedRequests = 0
	in.SuccessfulRequests = 3000
	in.TotalRequests = 3000
	in.StatusCodes = map[string]int64{"200": 3000}
	in.P50LatencyMs = 8.0
	in.P95LatencyMs = 12.0
	obs := Observe(in)
	joined := strings.Join(obs, "\n")
	for _, want := range []string{"unprotected baseline", "No failed requests", "stayed even"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("want %q in:\n%s", want, joined)
		}
	}
}

func TestObserveDefenseNeverEngaged(t *testing.T) {
	in := testInput()
	in.FailedRequests = 0
	in.SuccessfulRequests = 3000
	in.TotalRequests = 3000
	in.StatusCodes = map[string]int64{"200": 3000}
	joined := strings.Join(Observe(in), "\n")
	if !strings.Contains(joined, "never engaged") {
		t.Fatalf("want never-engaged note, got:\n%s", joined)
	}
}
