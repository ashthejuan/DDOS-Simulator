# Architecture — PDF experiment reports

**Date:** 2026-09-22  
**Related:** [prd.md](../prd.md) §14, [build-plan.md](../build-plan.md) Phase 6

## Decision

V1 experiment reports are **PDF-only** as the shareable artifact (plus optional JSON for the Oat on-page preview). HTML report export is dropped from V1.

## Why

- Demo / interviewer-friendly: one downloadable file with a clear visual hierarchy
- Stays in Go (no Chrome/Puppeteer → smaller Docker image)
- Matches “lightweight but polished” product bar

## Endpoint

```http
GET /api/experiments/:id/report.pdf
Content-Type: application/pdf
Content-Disposition: attachment; filename="ddoslab-experiment-{id}.pdf"
```

Optional: `GET /api/experiments/:id/report` → JSON used by `report.html` preview.

## Layout contract

1. Cover strip — DDoSLab + experiment id/date/status  
2. Config — duration, RPS, workers, defense on/off  
3. Traffic metrics — totals, peak RPS, success/fail, error rate  
4. Latency — avg, P50, P95, P99  
5. Server impact — CPU/memory if present, 4xx/5xx  
6. Observations — rule-based bullets (facts vs recommendations labeled)  
7. Footer — page # + local-lab disclaimer  

Visual rules: A4, ≥16mm margins, restrained accent, tables over dense charts, at most 1–2 simple figures.

## Implementation sketch (when built)

| Piece | Path (planned) |
| ----- | -------------- |
| PDF builder | `backend/internal/report/` |
| Handler | experiment routes → stream PDF bytes |
| UI | `web/report.html` — summary + Download PDF |

Prefer Maroto or gofpdf; no headless browser.

## Status

Spec only — generator not implemented yet.
