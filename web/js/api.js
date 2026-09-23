// Fetch helpers for the DDoSLab API (Phase 5).
// No framework; thin wrappers over fetch with JSON error extraction.
(function () {
  async function request(path, options) {
    const res = await fetch(path, options);
    const text = await res.text();
    let data = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      data = null;
    }
    if (!res.ok) {
      const msg =
        (data && data.error) || text || `HTTP ${res.status}`;
      throw new Error(`${options && options.method ? options.method : "GET"} ${path} -> ${res.status}: ${msg}`);
    }
    return data;
  }

  function get(path) {
    return request(path, { headers: { Accept: "application/json" } });
  }

  function post(path, body) {
    return request(path, {
      method: "POST",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: body === undefined ? null : JSON.stringify(body),
    });
  }

  function getHealth() {
    return get("/api/health");
  }

  function listExperiments() {
    return get("/api/experiments");
  }

  function getExperiment(id) {
    return get(`/api/experiments/${encodeURIComponent(id)}`);
  }

  function createExperiment(cfg) {
    return post("/api/experiments", cfg);
  }

  function stopExperiment(id) {
    return post(`/api/experiments/${encodeURIComponent(id)}/stop`);
  }

  function getMetrics(id) {
    return get(`/api/experiments/${encodeURIComponent(id)}/metrics`);
  }

  function reportPdfUrl(id) {
    // Phase 6 endpoint; present here so the Report screen can link it.
    return `/api/experiments/${encodeURIComponent(id)}/report.pdf`;
  }

  window.DDoSLabAPI = {
    getHealth,
    listExperiments,
    getExperiment,
    createExperiment,
    stopExperiment,
    getMetrics,
    reportPdfUrl,
  };
})();
