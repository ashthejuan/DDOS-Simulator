# Architecture — Stack & frontend decision

**Date:** 2026-09-22  
**Related:** [prd.md](../prd.md), [build-plan.md](../build-plan.md)

## What changed

V1 frontend moved from **React + Vite + Recharts** to **[Oat UI](https://oat.ink) + vanilla HTML/CSS/JS**.

## Why

Oat is a semantic, zero-dependency HTML/CSS/JS library (~10KB gzipped). It is **not** a React component library. Using it inside React would fight its design (classless semantic styling + Web Components) and reintroduce a Node build chain the project aims to avoid.

Decision: drop React for V1; serve static `web/` from the Go API.

## Runtime shape

```text
Browser ──► Go :8080 (REST + static Oat UI)
              ├──► Test server :8081 (local only)
              └──► MongoDB :27017
```

## Files (planned)

| Area | Path |
| ---- | ---- |
| Go API / workers | `backend/` |
| Oat UI | `web/` |
| Compose | `docker-compose.yml` |

## PRD edits

- Version bumped to 1.1
- Sections 1, 2, 5, 6, 15, 17–18, 20, 23–24, 26 updated for Oat
- Charts: tiny SVG / Oat meters instead of Recharts

## Not yet implemented

Code skeleton and phases in the build plan are pending; this note records the decision only.
