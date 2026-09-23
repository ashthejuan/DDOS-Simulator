// Dashboard: health, experiment list, create + stop (Phase 5).
(function () {
  const api = () => window.DDoSLabAPI;

  function shortId(id) {
    return id && id.length > 8 ? id.slice(0, 8) : id || "—";
  }

  async function checkHealth() {
    const out = document.getElementById("health-status");
    if (!out) return;
    out.textContent = "checking…";
    try {
      const data = await api().getHealth();
      out.textContent = data.status === "ok" ? "ok ●" : JSON.stringify(data);
    } catch (err) {
      out.textContent = `unreachable (${err.message})`;
    }
  }

  function rowHtml(e) {
    const cfg = e.config || {};
    const reqs = `${e.total_requests ?? 0} (${e.successful_requests ?? 0} ok / ${e.failed_requests ?? 0} fail)`;
    return `
      <td><code title="${e.id}">${shortId(e.id)}</code></td>
      <td>${cfg.endpoint ?? "—"}</td>
      <td>${cfg.duration_seconds ?? "—"}s</td>
      <td>${cfg.requests_per_second ?? "—"}</td>
      <td>${cfg.workers ?? "—"}</td>
      <td><span data-status="${e.status}">${e.status}</span></td>
      <td>${reqs}</td>
      <td class="actions">
        <a href="/live.html?id=${encodeURIComponent(e.id)}">Live</a>
        <a href="/report.html?id=${encodeURIComponent(e.id)}">Report</a>
        ${e.status === "running" ? `<button type="button" data-stop="${e.id}">Stop</button>` : ""}
      </td>`;
  }

  async function loadList() {
    const tbody = document.getElementById("exp-rows");
    const note = document.getElementById("list-note");
    try {
      const data = await api().listExperiments();
      const exps = (data && data.experiments) || [];
      if (exps.length === 0) {
        tbody.innerHTML = `<tr><td colspan="8">No experiments yet — start one above.</td></tr>`;
      } else {
        tbody.innerHTML = exps
          .map((e) => `<tr>${rowHtml(e)}</tr>`)
          .join("");
      }
      if (note) {
        const running = exps.filter((e) => e.status === "running").length;
        note.textContent = `${exps.length} experiment(s)${running ? ` · ${running} running` : ""}`;
      }
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="8">Failed to load: ${err.message}</td></tr>`;
    }
  }

  async function onCreate(ev) {
    ev.preventDefault();
    const out = document.getElementById("new-status");
    const form = ev.target;
    const cfg = {
      endpoint: form.endpoint.value,
      duration_seconds: Number(form.duration_seconds.value),
      requests_per_second: Number(form.requests_per_second.value),
      workers: Number(form.workers.value),
    };
    out.textContent = "Starting…";
    try {
      const created = await api().createExperiment(cfg);
      out.textContent = `Started ${created.id}`;
      form.reset();
      await loadList();
      window.location.href = `/live.html?id=${encodeURIComponent(created.id)}`;
    } catch (err) {
      out.textContent = `Error: ${err.message}`;
    }
  }

  async function onTableClick(ev) {
    const btn = ev.target.closest("[data-stop]");
    if (!btn) return;
    btn.disabled = true;
    try {
      await api().stopExperiment(btn.getAttribute("data-stop"));
      await loadList();
    } catch (err) {
      btn.disabled = false;
      btn.textContent = `Stop failed: ${err.message}`;
    }
  }

  function init() {
    document.getElementById("health-retry")?.addEventListener("click", checkHealth);
    document.getElementById("refresh")?.addEventListener("click", loadList);
    document.getElementById("new-form")?.addEventListener("submit", onCreate);
    document.getElementById("exp-rows")?.addEventListener("click", onTableClick);
    checkHealth();
    loadList();
    // Gentle auto-refresh while anything is running (5s; cheap GET).
    setInterval(async () => {
      const rows = document.getElementById("exp-rows");
      if (!rows || !rows.querySelector('[data-status="running"]')) return;
      await loadList();
    }, 5000);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
