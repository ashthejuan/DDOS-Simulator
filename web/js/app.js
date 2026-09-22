// Landing-page wiring: API health + minimal live metrics panel (Phase 3).
// Full dashboard screens arrive in Phase 5.
(async function () {
  const out = document.getElementById("health-status");
  const retry = document.getElementById("health-retry");

  async function check() {
    if (!out) return;
    out.textContent = "checking…";
    try {
      const data = await window.DDoSLabAPI.getHealth();
      out.textContent = data.status === "ok" ? "ok ●" : JSON.stringify(data);
    } catch (err) {
      out.textContent = `unreachable (${err.message})`;
    }
  }

  retry?.addEventListener("click", check);
  document.addEventListener("DOMContentLoaded", check);
  await check();

  // --- Live metrics panel: poll every 1.5s while the experiment runs. ---
  const idInput = document.getElementById("live-id");
  const toggle = document.getElementById("live-toggle");
  const table = document.getElementById("live-table");
  const note = document.getElementById("live-note");
  if (!toggle) return;

  let timer = null;
  const fmt = (n) => (n === null || n === undefined ? "n/a" : Number(n).toFixed(1));

  async function resolveId() {
    const typed = idInput?.value.trim();
    if (typed) return typed;
    const list = await window.DDoSLabAPI.listExperiments();
    const running = (list.experiments || []).find((e) => e.status === "running");
    if (!running) throw new Error("no running experiment — create one via POST /api/experiments");
    if (idInput) idInput.value = running.id;
    return running.id;
  }

  async function poll(id) {
    try {
      const m = await window.DDoSLabAPI.getMetrics(id);
      table.hidden = false;
      document.getElementById("m-status").textContent = `${m.status} (${m.elapsed_seconds.toFixed(0)}s)`;
      document.getElementById("m-rps").textContent = fmt(m.rps);
      document.getElementById("m-reqs").textContent =
        `${m.total_requests} (${m.successful_requests} / ${m.failed_requests}) ${JSON.stringify(m.status_codes)}`;
      document.getElementById("m-lat").textContent =
        `${fmt(m.avg_latency_ms)} / ${fmt(m.p50_latency_ms)} / ${fmt(m.p95_latency_ms)} / ${fmt(m.p99_latency_ms)}`;
      document.getElementById("m-sys").textContent =
        m.cpu_percent === null && m.memory_rss_bytes === null
          ? "n/a (non-Linux target)"
          : `${fmt(m.cpu_percent)}% / ${m.memory_rss_bytes === null ? "n/a" : (m.memory_rss_bytes / 1048576).toFixed(1) + " MiB"}`;
      if (m.status !== "running") stop(`finished: ${m.status}`);
    } catch (err) {
      stop(`error: ${err.message}`);
    }
  }

  function stop(msg) {
    if (timer) clearInterval(timer);
    timer = null;
    toggle.textContent = "Start live view";
    if (msg && note) note.textContent = msg;
  }

  toggle.addEventListener("click", async () => {
    if (timer) return stop();
    try {
      const id = await resolveId();
      toggle.textContent = "Stop live view";
      if (note) note.textContent = `Polling ${id} every 1.5s…`;
      await poll(id);
      if (timer) clearInterval(timer);
      timer = setInterval(() => poll(id), 1500);
    } catch (err) {
      if (note) note.textContent = `error: ${err.message}`;
    }
  });
})();
