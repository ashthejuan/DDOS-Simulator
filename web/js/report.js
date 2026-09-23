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
      const defense = cfg.defense ? "on — rate limiting" : "off — baseline";
      $("sum-rows").innerHTML =
        pair("Experiment", `<code>${exp.id}</code>`) +
        pair("Status", exp.status) +
        pair("Endpoint", `${exp.target_url || cfg.endpoint || "—"}`) +
        pair("Started", fmtTime(exp.started_at)) +
        pair("Completed", fmtTime(exp.completed_at)) +
        pair("Config", `${cfg.duration_seconds ?? "—"}s · ${cfg.requests_per_second ?? "—"} RPS · ${cfg.workers ?? "—"} workers · defense ${defense}`);

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

  function shortId(id) {
    return id && id.length > 8 ? id.slice(0, 8) : id || "?";
  }

  // Compare two runs side by side (UI only; comparison PDF is V2).
  async function compare(idA, idB) {
    const note = $("compare-note");
    const table = $("compare-table");
    if (!idA || !idB) {
      if (note) note.textContent = "Enter both experiment IDs first.";
      return;
    }
    if (note) note.textContent = "Loading…";
    try {
      const [a, b] = await Promise.all([
        Promise.all([window.DDoSLabAPI.getExperiment(idA), window.DDoSLabAPI.getMetrics(idA).catch(() => null)]),
        Promise.all([window.DDoSLabAPI.getExperiment(idB), window.DDoSLabAPI.getMetrics(idB).catch(() => null)]),
      ]);
      const rows = [
        ["Defense", (a[0].config || {}).defense ? "on" : "off", (b[0].config || {}).defense ? "on" : "off"],
        ["Status", a[0].status, b[0].status],
        ["Total requests", total(a[1], a[0]), total(b[1], b[0])],
        ["Error rate", rate(a[1], a[0]), rate(b[1], b[0])],
        ["P50 (ms)", lat(a[1], "p50_latency_ms"), lat(b[1], "p50_latency_ms")],
        ["P95 (ms)", lat(a[1], "p95_latency_ms"), lat(b[1], "p95_latency_ms")],
        ["P99 (ms)", lat(a[1], "p99_latency_ms"), lat(b[1], "p99_latency_ms")],
        ["HTTP 429", code(a[1], "429"), code(b[1], "429")],
        ["HTTP 5xx", fivexx(a[1]), fivexx(b[1])],
      ];
      $("cmp-a-h").textContent = `A (${shortId(idA)})`;
      $("cmp-b-h").textContent = `B (${shortId(idB)})`;
      $("compare-rows").innerHTML = rows
        .map(([k, x, y]) => `<tr><th scope="row">${k}</th><td>${x}</td><td>${y}</td></tr>`)
        .join("");
      table.hidden = false;
      if (note) note.textContent = "Measured values only — decide for yourself which trade-off is better.";
    } catch (err) {
      if (note) note.textContent = `error: ${err.message}`;
    }
  }

  function total(m, exp) {
    if (m) return String(m.total_requests);
    return String(exp.total_requests ?? "n/a");
  }

  function rate(m, exp) {
    const total = m ? m.total_requests : exp.total_requests;
    const fail = m ? m.failed_requests : exp.failed_requests;
    if (!total) return "n/a";
    return `${((fail / total) * 100).toFixed(1)}%`;
  }

  function lat(m, k) {
    if (!m || m[k] === null || m[k] === undefined) return "n/a";
    return Number(m[k]).toFixed(1);
  }

  function code(m, c) {
    if (!m || !m.status_codes) return "n/a";
    return String(m.status_codes[c] || 0);
  }

  function fivexx(m) {
    if (!m || !m.status_codes) return "n/a";
    return String(
      Object.entries(m.status_codes)
        .filter(([k]) => k.startsWith("5"))
        .reduce((n, [, v]) => n + v, 0)
    );
  }

  function init() {
    const params = new URLSearchParams(window.location.search);
    const qid = (params.get("id") || "").trim();
    const qcmp = (params.get("compare") || "").trim();
    if (qid) $("report-id").value = qid;
    if (qcmp) $("compare-id").value = qcmp;
    $("report-load")?.addEventListener("click", () => load($("report-id").value.trim()));
    $("compare-load")?.addEventListener("click", () =>
      compare($("report-id").value.trim(), $("compare-id").value.trim())
    );
    if (qid) load(qid);
    if (qid && qcmp) compare(qid, qcmp);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
