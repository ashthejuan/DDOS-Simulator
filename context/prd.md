# DDoSLab — Product Requirements Document

**Version:** 1.2
**Status:** Draft
**Target:** Beginner-friendly Go + Oat cybersecurity project
**Primary stack:** Go, Oat UI (vanilla HTML/CSS/JS), MongoDB, Docker
**Reports:** Go-generated PDF (clean A4 layout)

---

## 1. Product Overview

### 1.1 Product Name

**DDoSLab**

### 1.2 One-line description

DDoSLab is a lightweight, local cybersecurity learning application that simulates controlled high-volume HTTP traffic against a test server, collects performance metrics, and generates a report showing the effect of the traffic and the server's resilience.

### 1.3 Purpose

The project is designed to teach:

* Basic cybersecurity and DDoS concepts
* HTTP networking
* Concurrent programming in Go
* Backend API development
* Performance monitoring
* Rate limiting and defensive techniques
* MongoDB data modeling
* Semantic HTML / vanilla JS frontend (Oat UI)
* Basic security-oriented reporting

The application is strictly intended for **local, authorized testing environments**. The initial implementation should not provide functionality for attacking arbitrary external systems.

---

# 2. Goals

## 2.1 Primary Goals

The application should allow a user to:

1. Start a local test server.
2. Configure a controlled traffic simulation.
3. Run the simulation against the local test server.
4. Collect basic performance metrics.
5. View the experiment in real time through a lightweight Oat dashboard.
6. Stop the experiment.
7. Store experiment results in MongoDB.
8. Generate a clean PDF report after the experiment.
9. Compare experiments with and without basic defensive mechanisms.

---

## 2.2 Learning Goals

The project should deliberately expose the developer to:

### Go

* HTTP servers and clients
* Goroutines
* Channels
* `context.Context`
* Synchronization
* Concurrent workers
* HTTP timeouts
* JSON APIs
* Error handling
* Basic project structure

### Cybersecurity

* DoS vs DDoS concepts
* Traffic floods
* Rate limiting
* Request exhaustion
* Availability
* Basic detection
* Security telemetry
* Defensive mitigation

### Backend

* REST API design
* Background jobs
* Metrics collection
* MongoDB persistence
* Aggregation

### Frontend

* Semantic HTML with [Oat UI](https://oat.ink)
* Vanilla JS (no build step / no Node toolchain)
* `fetch` API requests and polling
* Simple SVG or meter-based charts
* Experiment configuration forms
* Results visualization

---

# 3. Non-Goals

The first version should **not** attempt to become a production DDoS platform.

Explicitly out of scope:

* Attacking public websites
* Distributed attacks against external IPs
* Botnet functionality
* IP spoofing
* UDP amplification
* SYN flooding
* Exploit development
* Malware
* CAPTCHA bypass
* Cloud-scale traffic generation
* Advanced packet manipulation
* Automatic discovery of attack targets
* Real-world attack orchestration

The simulator should operate against a **locally controlled test server**.

---

# 4. Target Users

### Primary user

A CS student/developer learning:

* Go
* Networking
* Cybersecurity
* Backend engineering

### Secondary user

An interviewer reviewing the project should be able to understand:

> "This person built a controlled traffic simulator, collected telemetry, analyzed system behavior, and implemented defensive experiments."

---

# 5. High-Level Architecture

```text
                 Oat Frontend (static)
                 HTML + CSS + vanilla JS
                          │
                          │ REST API
                          ▼
                 ┌─────────────────┐
                 │   Go Backend    │
                 │                 │
                 │ Experiment API  │
                 │ Traffic Engine  │
                 │ Metrics Engine  │
                 │ Report Engine   │
                 │ Static file serve│
                 └───────┬─────────┘
                         │
             ┌───────────┴───────────┐
             ▼                       ▼
      Local Test Server          MongoDB
             │                       │
             └───────────┬───────────┘
                         ▼
                   Experiment Data
```

**Frontend decision:** [Oat](https://oat.ink) is a zero-dependency semantic HTML/CSS/JS library (~10KB). It is not a React component library. To stay lightweight and match Oat’s design, V1 uses Oat + vanilla JS (no React, Vite, or npm frontend toolchain). The Go API serves the static `web/` assets.

For the first version, everything can run locally through Docker Compose.

---

# 6. Core User Flow

## 6.1 Create Experiment

User opens the Oat dashboard in the browser.

Dashboard displays:

```text
DDoSLab

Create Experiment

Target:
[ Local Test Server ▼ ]

Duration:
[ 60 seconds ]

Requests/sec:
[ 100 ]

Concurrent Workers:
[ 10 ]

[ Start Experiment ]
```

The target should initially be fixed to the application's own local test server.

---

## 6.2 Run Experiment

After starting:

```text
Experiment Running

Elapsed: 23s / 60s

Requests/sec       104
Successful         2,304
Failed                12

Latency
P50                  41ms
P95                 123ms
P99                 241ms

CPU                  64%
Memory               48%

[ Stop Experiment ]
```

The frontend periodically requests updated metrics from the Go backend.

---

## 6.3 Complete Experiment

When the experiment finishes:

```text
Experiment Complete

Peak RPS:             102
Average latency:       71ms
P95 latency:          184ms
Error rate:           0.8%

[ View Report ]
```

---

# 7. Experiment Configuration

The first version should expose only a few parameters.

### Required

| Parameter          | Description                              |
| ------------------ | ---------------------------------------- |
| Duration           | How long the experiment runs             |
| Requests/sec       | Approximate traffic rate                 |
| Concurrent workers | Number of Go workers generating requests |

### Optional

| Parameter       | Description                |
| --------------- | -------------------------- |
| Request method  | GET initially              |
| Endpoint        | Fixed local test endpoint  |
| Defense enabled | Enable local rate limiting |

Avoid exposing dozens of networking parameters.

The goal is learning, not building a sophisticated attack framework.

---

# 8. Traffic Simulation Engine

The Go backend contains a simple worker pool.

Conceptually:

```text
Experiment
    │
    ▼
Worker Pool
 ┌──┴──┬──┬──┬──┐
 ▼     ▼  ▼  ▼  ▼
 W1   W2 W3 W4 W5
 │     │  │  │  │
 └─────┴──┴──┴──┘
          │
          ▼
   Local Test Server
```

Each worker performs HTTP requests against the test server.

The number of workers should be configurable.

The simulator should respect the experiment's configured duration and stop cleanly using Go's `context.Context`.

---

# 9. Local Test Server

DDoSLab should contain its own intentionally simple HTTP server.

Example:

```http
GET /api/test
```

Response:

```json
{
  "status": "ok"
}
```

The test server should expose a small amount of artificial workload so that traffic has measurable effects.

For example:

```text
GET /api/test
GET /api/slow
```

`/api/slow` may intentionally perform a small amount of work or sleep for a configurable period.

The server should remain local and bundled with the application.

---

# 10. Metrics

The application should collect a small set of meaningful metrics.

### Traffic

* Total requests
* Requests/sec
* Successful requests
* Failed requests
* HTTP status codes

### Latency

* Average latency
* P50 latency
* P95 latency
* P99 latency

### Server health

* CPU usage
* Memory usage

### Experiment state

* Start time
* End time
* Duration
* Configuration
* Final status

---

# 11. MongoDB Data Model

Only a small number of collections are required.

## experiments

```json
{
  "_id": "...",
  "status": "completed",
  "duration_seconds": 60,
  "requests_per_second": 100,
  "workers": 10,
  "started_at": "...",
  "completed_at": "...",
  "total_requests": 6000,
  "successful_requests": 5940,
  "failed_requests": 60,
  "average_latency_ms": 72,
  "p50_latency_ms": 43,
  "p95_latency_ms": 184,
  "p99_latency_ms": 291
}
```

## metrics

Metrics can be stored periodically:

```json
{
  "experiment_id": "...",
  "timestamp": "...",
  "requests_per_second": 101,
  "average_latency_ms": 74,
  "p95_latency_ms": 190,
  "cpu_percent": 62,
  "memory_percent": 48
}
```

Do not over-engineer the schema.

---

# 12. Defensive Mode

A major learning feature is comparing an unprotected test server with a protected version.

### No protection

```text
Traffic
   ↓
Test Server
```

### Rate limiting

```text
Traffic
   ↓
Rate Limiter
   ↓
Test Server
```

The rate limiter can initially be a simple token-bucket or request-counting implementation in Go.

The user should be able to run:

```text
Experiment A
No rate limiting

        vs

Experiment B
Rate limiting enabled
```

---

# 13. Experiment Comparison

The frontend should allow users to select two completed experiments.

Example:

```text
Experiment A          Experiment B
No Protection         Rate Limited

Peak RPS     100      Peak RPS     100
P95          420ms    P95          130ms
Errors       8.2%     Errors       1.1%
```

The application should **display measured differences**, rather than automatically claiming that one configuration is universally better.

---

# 14. Report Generation

After an experiment, the Go backend should generate a **PDF report** as the primary deliverable.

Optional companion (V1 optional / V2): JSON export for programmatic reuse. HTML is not required if PDF covers the demo.

### Delivery

```http
GET /api/experiments/:id/report.pdf
```

Response: `application/pdf` attachment, e.g. `ddoslab-experiment-{id}.pdf`.

The Oat UI report screen shows a short on-page summary and a **Download PDF** action that hits this endpoint.

### Visual design (PDF)

Keep the PDF clean and interview-demo friendly — not a dense dump of numbers.

| Rule | Guidance |
| ---- | -------- |
| Page | A4 portrait, generous margins (≥16mm) |
| Brand | “DDoSLab” wordmark / title on cover strip; experiment ID + date as subtitle |
| Type | One sans family (embedded or standard PDF core fonts); clear hierarchy — title → section → body → caption |
| Color | Restrained palette: near-black text, cool gray rules, one accent for section headers / key metrics (avoid neon / purple glow) |
| Layout | Cover summary block → metric sections with hairline dividers → observations |
| Metrics | Present as labeled value pairs or simple 2-column tables; no cluttered multi-chart walls |
| Charts | At most 1–2 simple line/bar figures (RPS and/or latency over time) if data exists; skip charts rather than ship messy ones |
| Whitespace | Prefer open spacing over cramming; one primary idea per section |
| Footer | Page number + “Local lab report — not for production use” |

Library guidance (pick one pure-Go option, no headless browser):

* Prefer a layout-oriented Go PDF lib (e.g. Maroto or gofpdf) so typography and tables stay consistent.
* Do **not** shell out to Chrome/Puppeteer for V1 — keeps Docker image light.

### Report sections

#### Cover / Experiment Summary

```text
DDoSLab Experiment Report
Experiment ID · Date · Duration
Traffic configuration
Defense configuration (on/off)
Final status
```

#### Traffic

```text
Total requests
Peak requests/sec
Successful requests
Failed requests
Error rate
```

#### Performance

```text
Average latency
P50 · P95 · P99
```

#### Server Impact

```text
CPU (peak / avg if available)
Memory (peak / avg if available)
HTTP 4xx / 5xx counts
```

#### Observations

Simple rule-based observations:

```text
• Latency increased significantly during peak traffic.
• HTTP 5xx responses appeared during the experiment.
• Server performance returned toward baseline after traffic stopped.
```

The report should distinguish measured observations from recommendations.

---

# 15. Oat Frontend

The frontend should remain ultra-lightweight.

Stack:

* [Oat UI](https://oat.ink) (~7KB CSS + ~2.9KB JS gzipped)
* Static HTML pages (or a single-page shell with progressive enhancement)
* Vanilla JavaScript only — no React, Vite, Webpack, or npm build for the UI
* Oat CSS variables for light theme customization
* Simple SVG sparklines / Oat `<meter>` / progress for charts (no Recharts)

Vendor Oat via CDN or copy `oat.min.css` / `oat.min.js` into `web/vendor/`.

Go serves `web/` as static files from the same process as the API (one less container).

---

## 15.1 Pages

### Dashboard

Shows:

```text
Experiments
────────────────────────────

Recent Experiments

ID       Duration    RPS    Status
001      60s         100    Completed
002      30s         250    Completed
003      60s         500    Running
```

---

### New Experiment

Configuration form:

```text
Traffic Configuration

Duration
[ 60 ]

Requests/sec
[ 100 ]

Workers
[ 10 ]

Defense
○ Disabled
○ Rate Limiting

[ Start ]
```

---

### Live Experiment

Charts:

```text
Requests/sec
   ▲
   │       ╭───╮
   │   ╭───╯   ╰──
   └────────────────► time
```

And:

* Current RPS
* Latency
* Error rate
* CPU
* Memory
* Experiment progress

---

### Report

On-page preview (Oat):

```text
Experiment Report

Traffic
──────────────
Total Requests     6,000
Peak RPS             103

Latency
──────────────
P50                  43ms
P95                 184ms
P99                 291ms

Errors
──────────────
4xx                  32
5xx                  28

[ Download PDF ]
```

PDF is the canonical shareable artifact (see §14).

---

# 16. REST API

Keep the API small.

### Create experiment

```http
POST /api/experiments
```

Request:

```json
{
  "duration_seconds": 60,
  "requests_per_second": 100,
  "workers": 10,
  "rate_limiting": false
}
```

---

### Get experiment

```http
GET /api/experiments/:id
```

---

### Start experiment

```http
POST /api/experiments/:id/start
```

---

### Stop experiment

```http
POST /api/experiments/:id/stop
```

---

### Get live metrics

```http
GET /api/experiments/:id/metrics
```

---

### List experiments

```http
GET /api/experiments
```

---

### Get report (PDF)

```http
GET /api/experiments/:id/report.pdf
```

Returns `application/pdf`. Optional: `GET /api/experiments/:id/report` for JSON metadata used by the on-page preview.

---

# 17. Backend Project Structure

Keep the Go structure approachable.

```text
backend/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── experiment/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── model.go
│   │
│   ├── simulator/
│   │   └── simulator.go
│   │
│   ├── metrics/
│   │   └── metrics.go
│   │
│   ├── report/
│   │   └── report.go
│   │
│   ├── database/
│   │   └── mongodb.go
│   │
│   └── server/
│       └── test_server.go
│
├── go.mod
└── Dockerfile

web/                         # Oat + vanilla JS (served by Go)
├── index.html
├── css/
│   └── app.css              # thin overrides on Oat variables
├── js/
│   ├── api.js
│   ├── dashboard.js
│   ├── experiment.js
│   └── charts.js            # tiny SVG helpers
└── vendor/
    ├── oat.min.css
    └── oat.min.js
```

Do not introduce a complicated architecture pattern just for the sake of it.

---

# 18. Docker Architecture

A simple `docker-compose.yml` should run:

```text
┌──────────────────────┐
│  Go API + static web │
│  Oat UI on :8080     │
└───────┬──────────────┘
        │
        ├───────────────┐
        ▼               ▼
┌──────────────┐  ┌──────────────┐
│ Test Server  │  │   MongoDB    │
│    :8081     │  │    :27017    │
└──────────────┘  └──────────────┘
```

No separate Node/Vite frontend container in V1.

Redis should **not** be included in V1.

It adds complexity without providing much educational value for the first iteration.

---

# 19. Security & Safety Requirements

The application must enforce a local testing model.

### V1 restrictions

The simulator should only accept:

```text
localhost
127.0.0.1
Docker service name of the bundled test server
```

The frontend should not provide an arbitrary URL field.

The backend should validate the destination rather than trusting the frontend.

Example:

```text
Allowed:

http://localhost:8081
http://test-server:8081

Not allowed:

https://example.com
https://google.com
arbitrary external IP
```

This keeps the project focused on learning defensive security rather than creating an externally deployable traffic-generation tool.

---

# 20. MVP Scope

The **minimum viable version** should contain:

### Backend

* Go HTTP server
* MongoDB connection
* Experiment creation
* Local test server
* Concurrent HTTP workers
* Configurable duration
* Configurable request rate
* Basic metrics
* Experiment persistence
* PDF report generation (clean layout)

### Frontend

* Dashboard (Oat + semantic HTML)
* Experiment creation form
* Live experiment screen
* Basic SVG / meter charts
* Experiment history
* Report screen

### Infrastructure

* Docker Compose
* MongoDB
* Oat UI (static)
* Go

That's enough.

---

# 21. V2 Features

Only after V1 is complete:

### Security

* Better rate limiter
* IP-based detection
* Request burst detection
* Security alerts
* Traffic anomaly detection

### Analysis

* Experiment comparison
* Baseline vs peak comparison
* More detailed latency analysis

### Visualization

* RPS chart
* Latency chart
* Error-rate chart
* CPU/memory chart

### Reporting

* Comparison PDF (two experiments side-by-side)
* Downloadable JSON export
* Richer chart embeds in PDF

---

# 22. V3 / Advanced Features

These are intentionally optional.

* Reverse proxy mode
* Load-balancing experiments
* Multiple local containers acting as traffic sources
* Distributed local simulation
* WebSocket-based live metrics
* Prometheus integration
* Grafana integration
* More sophisticated anomaly detection

These should **not** be part of the initial implementation.

---

# 23. Suggested Development Milestones

## Milestone 1 — Go Fundamentals

Build:

```text
Go HTTP server
       ↓
GET /api/test
```

Learn:

* `net/http`
* handlers
* JSON
* structs
* error handling

---

## Milestone 2 — Traffic Simulator

Implement:

```text
N goroutines
      ↓
HTTP requests
      ↓
Local test server
```

Learn:

* Goroutines
* WaitGroups
* Channels
* Context cancellation

---

## Milestone 3 — Metrics

Collect:

```text
requests
successes
failures
latency
status codes
```

---

## Milestone 4 — MongoDB

Persist:

```text
experiments
metrics
```

---

## Milestone 5 — Oat UI

Build:

```text
Dashboard
   ↓
Experiment form
   ↓
Live metrics
   ↓
Report
```

Static HTML + Oat + vanilla JS, served by Go.

---

## Milestone 6 — Defensive Mode

Add:

```text
Rate limiting
```

Then run the same experiment with:

```text
Protection OFF
Protection ON
```

---

## Milestone 7 — Reporting

Generate PDF:

```text
Cover summary
Metrics tables
Optional simple chart(s)
Observations
Download from Oat UI
```

---

# 24. Definition of Done

V1 is complete when a user can:

1. Start the application using Docker Compose.
2. Open the Oat dashboard.
3. Create a traffic experiment.
4. Start the experiment.
5. See traffic and performance metrics.
6. Stop the experiment.
7. Have the results stored in MongoDB.
8. Open the completed experiment.
9. View / download a generated PDF report.
10. Run another experiment with rate limiting enabled.
11. Compare the measured results.

---

# 25. Example Final Demo

The ideal project demonstration would be:

```text
1. Start DDoSLab

2. Run:
   100 RPS
   30 seconds
   10 workers
   No protection

3. Show live dashboard

4. View / download PDF report

5. Enable rate limiting

6. Run the same experiment

7. Compare the two experiments

8. Explain:
   - How the Go workers work
   - How concurrency is controlled
   - How metrics are collected
   - How MongoDB stores experiments
   - How rate limiting changes behavior
   - What the measurements mean
```

This gives the project a clear story:

> **Generate controlled traffic → observe system behavior → apply a defensive mechanism → measure the difference.**

---

# 26. Technology Summary

| Component          | Technology                      |
| ------------------ | ------------------------------- |
| Frontend           | Oat UI + vanilla HTML/CSS/JS    |
| Backend            | Go                              |
| Database           | MongoDB                         |
| Charts             | Tiny SVG / Oat meters           |
| Containerization   | Docker + Docker Compose         |
| API                | REST                            |
| Traffic generation | Go HTTP client                  |
| Test server        | Go HTTP server                  |
| Reporting          | Go-generated PDF (clean layout) |
| Real-time updates  | HTTP polling initially          |
| Authentication     | Not required for V1             |
| Frontend build     | None (static files)             |

---

# 27. Guiding Principle

**Keep the implementation simple; make the concepts interesting.**

The project should not become a production-grade DDoS platform.

The value comes from understanding:

```text
Concurrency
     ↓
Network traffic
     ↓
System load
     ↓
Observability
     ↓
Detection
     ↓
Mitigation
     ↓
Measurement
```

That makes DDoSLab a compact project that demonstrates both **Go backend development and practical cybersecurity fundamentals** without requiring an enormous codebase.
