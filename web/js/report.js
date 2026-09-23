// Report screen: on-page preview + Download PDF link (Phase 5).
// PDF bytes land in Phase 6; the link targets that endpoint already.
(function () {
  const $ = (id) => document.getElementById(id);
  const fmt = (n, d) =>
    n === null || n === undefined || !Number.isFinite(Number(n))
      ? "n/a"
      : Number(n).toFixed(d === undefined ? 1 : d);

  function pair(k, v) {
    return `<tr><th scope="row">${k}</th><td>${v}</td></tr>`;
  }

  function idFromQuery() {
    const q = new URLSearchParams(window.location.search).get("id");
    return q && q.trim() ? q.trim() : "";
  }

  function fmtTime(t) {
    if (!t) return "—";
    try {
      return new Date(t).toLocaleString();
    } catch {
      return String(t);
    }
  }

  async function load(id) {
    const note = $("report-note");
    if (!id) {
      if (note) note.textContent = "Enter an experiment ID first.";
      return;
    }
    if (note) note.textContent = `Loading ${id}…`;
    try {
      const [exp, m] = await Promise.all([
        window.DDoSLabAPI.getExperiment(id),
        window.DDoSLabAPI.getMetrics(id).catch(() => null),
      ]);
      const cfg = exp.config || {};
      $("sum-rows").innerHTML =
        pair("Experiment", `<code>${exp.id}</code>`) +
        pair("Status", exp.status) +
        pair("Endpoint", `${exp.target_url || cfg.endpoint || "—"}`) +
        pair("Started", fmtTime(exp.started_at)) +
        pair("Completed", fmtTime(exp.completed_at)) +
        pair("Config", `${cfg.duration_seconds ?? "—"}s · ${cfg.requests_per_second ?? "—"} RPS · ${cfg.workers ?? "—"} workers`);

      const total = m ? m.total_requests : exp.total_requests;
      const ok = m ? m.successful_requests : exp.successful_requests;
      const fail = m ? m.failed_requests : exp.failed_requests;
      const errRate = total > 0 ? ((fail / total) * 100).toFixed(1) + "%" : "n/a";
      $("traffic-rows").innerHTML =
        pair("Total requests", total ?? "—") +
        pair("Successful", ok ?? "—") +
        pair("Failed", fail ?? "—") +
        pair("Error rate", errRate) +
        pair("RPS", m ? fmt(m.rps) : "n/a (run finished; live RPS unavailable)");

      $("latency-rows").innerHTML = m
        ? pair("Average", `${fmt(m.avg_latency_ms)} ms`) +
          pair("P50", `${fmt(m.p50_latency_ms)} ms`) +
          pair("P95", `${fmt(m.p95_latency_ms)} ms`) +
          pair("P99", `${fmt(m.p99_latency_ms)} ms`)
        : `<tr><td>Metrics unavailable for this experiment.</td></tr>`;

      const codes = m ? m.status_codes || {} : {};
      const codeRows = Object.keys(codes).length
        ? Object.keys(codes)
            .sort()
            .map((c) => pair(`HTTP ${c}`, codes[c]))
            .join("")
        : pair("Status codes", "none recorded");
      const sys = m
        ? pair(
            "Target CPU / RSS",
            m.cpu_percent === null && m.memory_rss_bytes === null
              ? "n/a (non-Linux target)"
              : `${fmt(m.cpu_percent)}% / ${m.memory_rss_bytes === null ? "n/a" : (m.memory_rss_bytes / 1048576).toFixed(1) + " MiB"}`
          )
        : "";
      $("error-rows").innerHTML = codeRows + sys;

      const pdf = $("pdf-link");
      pdf.href = window.DDoSLabAPI.reportPdfUrl(id);
      pdf.setAttribute("download", `ddoslab-experiment-${id}.pdf`);
      const back = $("back-live");
      if (back) back.href = `/live.html?id=${encodeURIComponent(id)}`;
      if (note) note.textContent = `Showing ${id} (${exp.status}).`;
    } catch (err) {
      if (note) note.textContent = `error: ${err.message}`;
    }
  }

  function init() {
    const qid = idFromQuery();
    if (qid) $("report-id").value = qid;
    $("report-load")?.addEventListener("click", () => load($("report-id").value.trim()));
    if (qid) load(qid);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
