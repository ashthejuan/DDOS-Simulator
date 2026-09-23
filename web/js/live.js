// Live experiment view: poll metrics every 1.5s, meters + SVG sparklines.
(function () {
  const POLL_MS = 1500;
  const KEEP = 60; // ~90s of history at 1.5s cadence
  let timer = null;
  let currentId = null;
  const rpsHist = [];
  const p95Hist = [];

  const $ = (id) => document.getElementById(id);
  const fmt = (n, d) =>
    n === null || n === undefined || !Number.isFinite(Number(n))
      ? "n/a"
      : Number(n).toFixed(d === undefined ? 1 : d);

  function idFromQuery() {
    const q = new URLSearchParams(window.location.search).get("id");
    return q && q.trim() ? q.trim() : null;
  }

  async function resolveId(typed) {
    if (typed) return typed;
    const list = await window.DDoSLabAPI.listExperiments();
    const running = ((list && list.experiments) || []).find((e) => e.status === "running");
    if (!running) throw new Error("no running experiment — start one from the dashboard");
    return running.id;
  }

  async function poll() {
    if (!currentId) return;
    const note = $("live-note");
    try {
      const [m, exp] = await Promise.all([
        window.DDoSLabAPI.getMetrics(currentId),
        window.DDoSLabAPI.getExperiment(currentId).catch(() => null),
      ]);
      $("m-status").textContent = m.status;
      $("m-elapsed").textContent = `${fmt(m.elapsed_seconds, 0)}s`;
      $("m-rps").textContent = fmt(m.rps);
      $("m-reqs").textContent =
        `${m.total_requests} · ${m.successful_requests} ok / ${m.failed_requests} fail`;
      $("m-lat").textContent =
        `${fmt(m.avg_latency_ms)} / ${fmt(m.p50_latency_ms)} / ${fmt(m.p95_latency_ms)} / ${fmt(m.p99_latency_ms)}`;
      $("m-sys").textContent =
        m.cpu_percent === null && m.memory_rss_bytes === null
          ? "n/a (non-Linux target)"
          : `${fmt(m.cpu_percent)}% / ${m.memory_rss_bytes === null ? "n/a" : (m.memory_rss_bytes / 1048576).toFixed(1) + " MiB"}`;
      $("m-codes").textContent = JSON.stringify(m.status_codes || {});

      // Meters: RPS vs configured target; error share 0..1.
      const target = exp && exp.config ? Number(exp.config.requests_per_second) : 0;
      const rpsMeter = $("meter-rps");
      rpsMeter.max = target > 0 ? target : Math.max(1, Math.ceil(m.rps));
      rpsMeter.value = m.rps;
      $("meter-rps-t").textContent = target > 0 ? `${fmt(m.rps)} / ${target} target` : fmt(m.rps);
      const errShare = m.total_requests > 0 ? m.failed_requests / m.total_requests : 0;
      $("meter-err").value = errShare;
      $("meter-err-t").textContent = `${(errShare * 100).toFixed(1)}% errors`;

      window.DDoSLabCharts.pushHistory(rpsHist, m.rps, KEEP);
      window.DDoSLabCharts.pushHistory(p95Hist, m.p95_latency_ms, KEEP);
      window.DDoSLabCharts.sparkline($("spark-rps"), rpsHist);
      window.DDoSLabCharts.sparkline($("spark-p95"), p95Hist);

      const link = $("live-report");
      if (link) link.href = `/report.html?id=${encodeURIComponent(currentId)}`;
      if (note) note.textContent = `Polling ${currentId} every 1.5s… (${m.status})`;
      if (m.status !== "running") stop(`finished: ${m.status} — open the report.`);
    } catch (err) {
      stop(`error: ${err.message}`);
    }
  }

  function stop(msg) {
    if (timer) clearInterval(timer);
    timer = null;
    const toggle = $("live-toggle");
    if (toggle) toggle.textContent = "Start live view";
    if (msg && $("live-note")) $("live-note").textContent = msg;
  }

  async function start() {
    const toggle = $("live-toggle");
    try {
      const typed = ($("live-id") || {}).value ? $("live-id").value.trim() : "";
      currentId = await resolveId(typed || idFromQuery());
      $("live-id").value = currentId;
      rpsHist.length = 0;
      p95Hist.length = 0;
      if (toggle) toggle.textContent = "Stop live view";
      await poll();
      if (timer) clearInterval(timer);
      if (currentId) timer = setInterval(poll, POLL_MS);
    } catch (err) {
      if ($("live-note")) $("live-note").textContent = `error: ${err.message}`;
    }
  }

  async function stopExperiment() {
    if (!currentId) return;
    try {
      await window.DDoSLabAPI.stopExperiment(currentId);
      await poll();
    } catch (err) {
      if ($("live-note")) $("live-note").textContent = `stop failed: ${err.message}`;
    }
  }

  function init() {
    const qid = idFromQuery();
    if (qid && $("live-id")) $("live-id").value = qid;
    $("live-toggle")?.addEventListener("click", () => {
      if (timer) return stop();
      start();
    });
    $("live-stop")?.addEventListener("click", stopExperiment);
    // Autostart when ?id= is present.
    if (qid) start();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
