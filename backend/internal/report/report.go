// Package report builds the Phase 6 PDF experiment report.
//
// Pure-Go rendering via go-pdf/fpdf (the maintained gofpdf continuation):
// no headless browser, A4 portrait, generous margins, one accent color,
// labeled value pairs over dense charts. See context/architecture/01-pdf-reports.md.
package report

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
)

// Input is the report data contract. The experiment handler maps its Result
// onto this DTO so this package never imports the experiment package
// (which would be an import cycle via the handler).
type Input struct {
	ID                 string
	Status             string
	StartedAt          time.Time
	CompletedAt        *time.Time // nil while running
	TargetURL          string
	Defense            bool
	DurationSeconds    int
	RequestsPerSecond  int
	Workers            int
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	RPS                float64
	AvgLatencyMs       float64
	P50LatencyMs       float64
	P95LatencyMs       float64
	P99LatencyMs       float64
	Samples            int
	StatusCodes        map[string]int64
	CPUPercent         *float64
	MemoryRSSBytes     *int64
}

const (
	accentR, accentG, accentB = 37, 99, 235 // restrained blue
	marginMM                 = 18.0
)

// Build renders res as a complete PDF document.
func Build(res Input) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginMM, 18, marginMM)
	pdf.SetAutoPageBreak(true, 20)
	pdf.SetTitle(fmt.Sprintf("DDoSLab Experiment Report — %s", res.ID), false)
	pdf.SetAuthor("DDoSLab", false)

	pdf.AddPage()
	footer(pdf)

	coverStrip(pdf, res)
	section(pdf, "Traffic configuration")
	pairs(pdf, configRows(res))
	rule(pdf)

	section(pdf, "Traffic")
	pairs(pdf, trafficRows(res))
	rule(pdf)

	section(pdf, "Performance")
	pairs(pdf, latencyRows(res))
	rule(pdf)

	section(pdf, "Server impact")
	pairs(pdf, serverRows(res))
	rule(pdf)

	section(pdf, "Observations")
	for _, o := range Observe(res) {
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetTextColor(30, 30, 30)
		pdf.MultiCell(0, 5.5, "•  "+o, "", "L", false)
		pdf.Ln(1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Observe derives measured, rule-based statements from a result. It reports
// what happened (facts); any mitigation reading is labeled as such.
func Observe(res Input) []string {
	var out []string
	m := res
	total := m.TotalRequests
	fail := res.FailedRequests
	if total < 0 {
		total = 0
	}

	limited := m.StatusCodes["429"]
	fivexx := sumCodes(m.StatusCodes, 500, 599)
	fourxx := sumCodes(m.StatusCodes, 400, 499) - limited

	defense := "off"
	if res.Defense {
		defense = "on"
	}

	if res.Defense {
		if limited > 0 {
			out = append(out, fmt.Sprintf(
				"Defense was on: the rate limiter rejected %d excess request(s) with HTTP 429 (%.1f%% of total).",
				limited, pct(limited, total)))
		} else {
			out = append(out, "Defense was on but never engaged — traffic stayed under the rate limit (no HTTP 429).")
		}
	} else {
		out = append(out, fmt.Sprintf("Defense was %s: this run is an unprotected baseline.", defense))
	}

	if fivexx > 0 {
		out = append(out, fmt.Sprintf("HTTP 5xx responses appeared during the experiment (%d).", fivexx))
	}
	if fourxx > 0 {
		out = append(out, fmt.Sprintf("HTTP 4xx (non-429) responses appeared (%d) — possible overload or client errors.", fourxx))
	}
	if fail == 0 {
		out = append(out, "No failed requests recorded.")
	} else if !res.Defense || limited < fail {
		out = append(out, fmt.Sprintf("Error rate was %.1f%% (%d of %d requests).", pct(fail, total), fail, total))
	}

	if m.P50LatencyMs > 0 && m.P95LatencyMs >= 3*m.P50LatencyMs {
		out = append(out, fmt.Sprintf(
			"Tail latency inflated: P95 (%.1f ms) was well above P50 (%.1f ms) under load.",
			m.P95LatencyMs, m.P50LatencyMs))
	} else if m.P50LatencyMs > 0 {
		out = append(out, fmt.Sprintf(
			"Latency stayed even: P50 %.1f ms, P95 %.1f ms.",
			m.P50LatencyMs, m.P95LatencyMs))
	}
	if total == 0 {
		out = append(out, "No requests completed — check that the test server was reachable.")
	}
	return out
}

func configRows(res Input) [][2]string {
	c := res
	defense := "off — unprotected baseline"
	if c.Defense {
		defense = "on — rate limiting"
	}
	return [][2]string{
		{"Endpoint", res.TargetURL},
		{"Duration", fmt.Sprintf("%d s", c.DurationSeconds)},
		{"Target rate", fmt.Sprintf("%d req/s", c.RequestsPerSecond)},
		{"Workers", fmt.Sprintf("%d", c.Workers)},
		{"Defense", defense},
		{"Final status", res.Status},
	}
}

func trafficRows(res Input) [][2]string {
	m := res
	total := m.TotalRequests
	elapsed := elapsedSeconds(res)
	avgRPS := 0.0
	if elapsed > 0 {
		avgRPS = float64(total) / elapsed
	}
	return [][2]string{
		{"Total requests", fmt.Sprintf("%d", total)},
		{"Successful", fmt.Sprintf("%d", res.SuccessfulRequests)},
		{"Failed", fmt.Sprintf("%d", res.FailedRequests)},
		{"Error rate", fmt.Sprintf("%.1f%%", pct(res.FailedRequests, total))},
		{"Average rate", fmt.Sprintf("%.1f req/s (total / elapsed)", avgRPS)},
		{"Rate at completion", fmt.Sprintf("%.1f req/s (trailing 5 s window)", m.RPS)},
	}
}

func latencyRows(res Input) [][2]string {
	m := res
	return [][2]string{
		{"Average", fmt.Sprintf("%.1f ms", m.AvgLatencyMs)},
		{"P50", fmt.Sprintf("%.1f ms", m.P50LatencyMs)},
		{"P95", fmt.Sprintf("%.1f ms", m.P95LatencyMs)},
		{"P99", fmt.Sprintf("%.1f ms", m.P99LatencyMs)},
		{"Samples", fmt.Sprintf("%d", m.Samples)},
	}
}

func serverRows(res Input) [][2]string {
	rows := [][2]string{}
	if res.CPUPercent != nil {
		rows = append(rows, [2]string{"Target CPU", fmt.Sprintf("%.1f%%", *res.CPUPercent)})
	} else {
		rows = append(rows, [2]string{"Target CPU", "n/a (non-Linux target)"})
	}
	if res.MemoryRSSBytes != nil {
		rows = append(rows, [2]string{"Target memory (RSS)", fmt.Sprintf("%.1f MiB", float64(*res.MemoryRSSBytes)/1048576)})
	} else {
		rows = append(rows, [2]string{"Target memory (RSS)", "n/a (non-Linux target)"})
	}
	codes := res.StatusCodes
	if len(codes) == 0 {
		rows = append(rows, [2]string{"Status codes", "none recorded"})
		return rows
	}
	keys := make([]string, 0, len(codes))
	for k := range codes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		rows = append(rows, [2]string{"HTTP " + k, fmt.Sprintf("%d", codes[k])})
	}
	return rows
}

// --- layout primitives ---

func footer(pdf *fpdf.Fpdf) {
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(130, 130, 130)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Local lab report — not for production use  ·  page %d", pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
}

func coverStrip(pdf *fpdf.Fpdf, res Input) {
	pdf.SetFillColor(accentR, accentG, accentB)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 20)
	pdf.CellFormat(0, 12, "DDoSLab Experiment Report", "", 1, "L", true, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 7,
		fmt.Sprintf("Experiment %s  ·  %s  ·  %s",
			res.ID, res.StartedAt.Format(time.RFC3339), res.Status),
		"", 1, "L", true, 0, "")
	pdf.Ln(6)
}

func section(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetTextColor(accentR, accentG, accentB)
	pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
	pdf.Ln(1)
}

func pairs(pdf *fpdf.Fpdf, rows [][2]string) {
	for _, r := range rows {
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(62, 6.5, r[0], "", 0, "L", false, 0, "")
		pdf.SetTextColor(20, 20, 20)
		pdf.CellFormat(0, 6.5, r[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(2)
}

func rule(pdf *fpdf.Fpdf) {
	x := pdf.GetX()
	pdf.SetDrawColor(220, 220, 220)
	pdf.Line(x, pdf.GetY(), 210-marginMM, pdf.GetY())
	pdf.Ln(4)
}

func elapsedSeconds(res Input) float64 {
	end := time.Now().UTC()
	if res.CompletedAt != nil {
		end = *res.CompletedAt
	}
	return end.Sub(res.StartedAt).Seconds()
}

func pct(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func sumCodes(codes map[string]int64, from, to int) int64 {
	var n int64
	for k, v := range codes {
		var c int
		if _, err := fmt.Sscanf(k, "%d", &c); err == nil && c >= from && c <= to {
			n += v
		}
	}
	return n
}
