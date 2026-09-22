// Fetch helpers for the DDoSLab API.
async function apiGet(path) {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (!res.ok) throw new Error(`GET ${path} -> ${res.status}`);
  return res.json();
}

async function getHealth() {
  return apiGet("/api/health");
}

async function listExperiments() {
  return apiGet("/api/experiments");
}

async function getMetrics(id) {
  return apiGet(`/api/experiments/${encodeURIComponent(id)}/metrics`);
}

window.DDoSLabAPI = { apiGet, getHealth, listExperiments, getMetrics };
