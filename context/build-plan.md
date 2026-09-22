# DDoSLab — Build Plan

**Status:** Active  
**PRD:** [prd.md](./prd.md)  
**UI:** [Oat UI](https://oat.ink) (vanilla HTML/CSS/JS — not React)  
**Principle:** Smallest useful slice per phase. No frontend build tool. Go serves static `web/`.

---

## Stack (locked)

| Layer | Choice | Why |
| ----- | ------ | --- |
| UI | Oat + vanilla JS | ~10KB, zero deps, no Node toolchain |
| API / workers / test server | Go | Learning goal + concurrency |
| Reports | Go PDF (Maroto/gofpdf) | Clean A4 download; no headless browser |
| DB | MongoDB | Persist experiments + metrics |
| Run | Docker Compose | Local-only, 2–3 services |

Out: React, Vite, Recharts, Redis, npm UI apps, Chrome/HTML→PDF.

---

## Repo layout (target)

```text
├── backend/          # Go API, simulator, metrics, report, test server
├── web/              # Oat static UI (served by Go on :8080)
├── docker-compose.yml
└── context/          # PRD, this plan, architecture notes
```

---

## Phases

### Phase 0 — Skeleton (½ day)

- [x] `docker-compose.yml`: `api` (Go) + `mongodb` (+ optional `test-server` or same binary)
- [x] Go module + `cmd/server` that listens on `:8080`
- [x] Serve `web/` as static files (`index.html` placeholder)
- [x] Vendor Oat (`web/vendor/oat.min.css`, `oat.min.js`) or CDN for local dev
- [x] Health: `GET /api/health`

**Done when:** Compose up → browser shows Oat-styled page from Go.

---

### Phase 1 — Local test server

- [x] Bundled HTTP target (`:8081`): `GET /api/test`, `GET /api/slow`
- [x] Optional artificial work / sleep so load is measurable
- [x] Backend allowlist: only localhost / compose service name

**Done when:** `curl` to test server returns JSON; external hosts rejected by config.

---

### Phase 2 — Experiment API + workers

- [x] Models: experiment config + status
- [x] `POST /api/experiments` → start worker pool (goroutines + `context`)
- [x] `POST /api/experiments/:id/stop`
- [x] `GET /api/experiments`, `GET /api/experiments/:id`
- [x] Workers: duration, RPS target, concurrency; HTTP client timeouts

**Done when:** Create experiment → workers hit local test server → stop cleanly.

---

### Phase 3 — Metrics + live poll

- [ ] Counters: total / success / fail / status codes
- [ ] Latency: avg, P50, P95, P99 (simple in-memory histogram or sorted samples)
- [ ] Optional: CPU/memory of test process (keep crude if hard)
- [ ] `GET /api/experiments/:id/metrics`
- [ ] UI: poll every 1–2s while `running`

**Done when:** Live page updates RPS / latency / errors during a run.

---

### Phase 4 — MongoDB

- [ ] Connect from Go; collections `experiments`, `metrics` (as in PRD)
- [ ] Persist on complete/stop; list history on dashboard
- [ ] Idempotent indexes if needed (`_id`, `started_at`)

**Done when:** Restart API → past experiments still listed.

---

### Phase 5 — Oat UI screens

Minimal pages (semantic HTML + Oat; thin `app.css` overrides only):

| Screen | File / route | Behavior |
| ------ | ------------ | -------- |
| Dashboard | `index.html` | List experiments table |
| New | form section or `new.html` | Duration, RPS, workers, defense toggle → POST |
| Live | `live.html?id=` | Poll metrics; meters + tiny SVG sparkline |
| Report | `report.html?id=` | On-page summary + **Download PDF** |

- [ ] `web/js/api.js` — fetch helpers
- [ ] No SPA framework; multi-page or hash routes OK
- [ ] Charts: Oat `<meter>` / `<progress>` + ~50-line SVG helper (no chart lib)

**Done when:** Full flow works from browser with no Node install.

---

### Phase 6 — Defense mode + PDF report

- [ ] Rate limiting on test server (toggle via experiment flag)
- [ ] Run same config with defense off vs on
- [ ] Go PDF generator (Maroto or gofpdf): A4, clean typography, sectioned metrics, observations
- [ ] `GET /api/experiments/:id/report.pdf` → `application/pdf`
- [ ] Optional: `GET /api/experiments/:id/report` JSON for on-page preview
- [ ] Optional: side-by-side compare of two experiment IDs (UI only; comparison PDF is V2)

**Done when:** Demo script in PRD §25 works end-to-end; PDF opens cleanly in a viewer.

---

## Order of work (critical path)

```text
Skeleton → Test server → Workers/API → Metrics → Mongo → Oat UI → Defense/PDF report
```

UI can start after Phase 0 (static shell), but wire real data only after Phase 3.

---

## Explicit non-work (V1)

- React / Vite / TypeScript frontend
- WebSockets (polling is enough)
- Auth, CAPTCHA
- Headless Chrome / HTML→PDF pipelines
- Attacking non-local targets
- Redis, message queues, microservices split

---

## Definition of done (V1)

Matches PRD §24: Compose up → Oat dashboard → create/run/stop → live metrics → Mongo persistence → **download clean PDF report** → defense on/off comparison.

---

## Architecture notes

Implementation details per phase live under [architecture/](./architecture/). Update those files when a phase lands.
