// Fetch helpers for the DDoSLab API (Phase 0: health only).
// Later phases add experiment + metrics helpers here.
async function apiGet(path) {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (!res.ok) throw new Error(`GET ${path} -> ${res.status}`);
  return res.json();
}

async function getHealth() {
  return apiGet("/api/health");
}

window.DDoSLabAPI = { apiGet, getHealth };
