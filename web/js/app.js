// Phase 0 placeholder wiring: show API health on the landing page.
(async function () {
  const out = document.getElementById("health-status");
  const retry = document.getElementById("health-retry");
  if (!out) return;

  async function check() {
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
})();
