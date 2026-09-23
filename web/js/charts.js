// Tiny SVG helpers (Phase 5). No chart lib: Oat <meter>/<progress> for
// gauges plus a ~50-line sparkline renderer for RPS / latency history.
(function () {
  const NS = "http://www.w3.org/2000/svg";

  function el(name, attrs) {
    const n = document.createElementNS(NS, name);
    for (const k of Object.keys(attrs || {})) n.setAttribute(k, attrs[k]);
    return n;
  }

  // sparkline(svg, values, opts): draws a line + end dot + min/max labels.
  // svg must have a viewBox; we default to "0 0 300 64".
  // opts: {stroke, width, min, max}
  function sparkline(svg, values, opts) {
    const o = Object.assign({ stroke: "currentColor", width: 2 }, opts || {});
    while (svg.firstChild) svg.removeChild(svg.firstChild);
    if (!svg.getAttribute("viewBox")) svg.setAttribute("viewBox", "0 0 300 64");
    const W = 300;
    const H = 64;
    const PAD = 6;
    const vals = (values || []).filter((v) => Number.isFinite(v));
    if (vals.length === 0) {
      const t = document.createElementNS(NS, "text");
      t.setAttribute("x", "8");
      t.setAttribute("y", "32");
      t.setAttribute("class", "spark-empty");
      t.textContent = "no data yet";
      svg.appendChild(t);
      return { min: 0, max: 0 };
    }
    let min = o.min !== undefined ? o.min : Math.min.apply(null, vals);
    let max = o.max !== undefined ? o.max : Math.max.apply(null, vals);
    if (min === max) {
      min -= 1;
      max += 1;
    }
    const x = (i) =>
      vals.length === 1
        ? W / 2
        : PAD + (i * (W - PAD * 2)) / (vals.length - 1);
    const y = (v) => H - PAD - ((v - min) / (max - min)) * (H - PAD * 2);
    const d = vals
      .map((v, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(v).toFixed(1)}`)
      .join(" ");
    svg.appendChild(el("path", { d, fill: "none", stroke: o.stroke, "stroke-width": o.width, "stroke-linejoin": "round", "stroke-linecap": "round" }));
    const last = vals[vals.length - 1];
    svg.appendChild(el("circle", { cx: x(vals.length - 1), cy: y(last), r: 3, fill: o.stroke }));
    return { min, max };
  }

  // pushHistory(arr, v, max): bounded ring buffer for poll series.
  function pushHistory(arr, v, max) {
    arr.push(v);
    while (arr.length > max) arr.shift();
    return arr;
  }

  window.DDoSLabCharts = { sparkline, pushHistory };
})();
