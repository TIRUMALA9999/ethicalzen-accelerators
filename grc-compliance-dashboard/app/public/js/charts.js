// Lightweight Canvas-based Charts

const Charts = {

  // Track last rendered chart params for re-rendering on theme change
  _rendered: [],

  _trackRender(type, args) {
    // Remove any existing entry for the same canvasId to avoid duplicates
    this._rendered = this._rendered.filter(r => r.args[0] !== args[0]);
    this._rendered.push({ type, args });
  },

  reRenderAll() {
    // Re-render all tracked charts with their last params (for theme change)
    for (const { type, args } of this._rendered) {
      this[type](...args);
    }
  },

  // Donut chart for compliance coverage
  donut(canvasId, value, total, color = 'var(--primary)') {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width = canvas.offsetWidth * 2;
    const h = canvas.height = canvas.offsetHeight * 2;
    ctx.scale(2, 2);

    const cx = canvas.offsetWidth / 2;
    const cy = canvas.offsetHeight / 2;
    const r = Math.min(cx, cy) - 8;
    const pct = total > 0 ? value / total : 0;
    const lineWidth = 10;

    // Background ring
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--border').trim();
    ctx.lineWidth = lineWidth;
    ctx.stroke();

    // Value ring
    ctx.beginPath();
    ctx.arc(cx, cy, r, -Math.PI / 2, -Math.PI / 2 + Math.PI * 2 * pct);
    ctx.strokeStyle = color;
    ctx.lineWidth = lineWidth;
    ctx.lineCap = 'round';
    ctx.stroke();

    // Center text
    ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--text-primary').trim();
    ctx.font = `bold ${r * 0.5}px Inter, sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(`${Math.round(pct * 100)}%`, cx, cy);
    this._trackRender('donut', [canvasId, value, total, color]);
  },

  // Bar chart for violation timeline
  barChart(canvasId, data, options = {}) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width = canvas.offsetWidth * 2;
    const h = canvas.height = canvas.offsetHeight * 2;
    ctx.scale(2, 2);

    const cw = canvas.offsetWidth;
    const ch = canvas.offsetHeight;
    const pad = { top: 10, right: 10, bottom: 30, left: 40 };

    if (!data.length) return;

    const max = Math.max(...data.map(d => d.value), 1);
    const barW = (cw - pad.left - pad.right) / data.length - 4;

    // Y axis
    ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--border').trim();
    ctx.lineWidth = 0.5;
    for (let i = 0; i <= 4; i++) {
      const y = pad.top + (ch - pad.top - pad.bottom) * (1 - i / 4);
      ctx.beginPath();
      ctx.moveTo(pad.left, y);
      ctx.lineTo(cw - pad.right, y);
      ctx.stroke();

      ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--text-muted').trim();
      ctx.font = '10px Inter, sans-serif';
      ctx.textAlign = 'right';
      ctx.fillText(Math.round(max * i / 4), pad.left - 6, y + 3);
    }

    // Bars
    data.forEach((d, i) => {
      const x = pad.left + i * ((cw - pad.left - pad.right) / data.length) + 2;
      const barH = (d.value / max) * (ch - pad.top - pad.bottom);
      const y = ch - pad.bottom - barH;

      ctx.fillStyle = d.color || options.color || getComputedStyle(document.documentElement).getPropertyValue('--primary').trim();
      ctx.beginPath();
      ctx.roundRect(x, y, barW, barH, [3, 3, 0, 0]);
      ctx.fill();

      // Label
      if (d.label) {
        ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--text-muted').trim();
        ctx.font = '10px Inter, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(d.label, x + barW / 2, ch - pad.bottom + 14);
      }
    });
    this._trackRender('barChart', [canvasId, data, options]);
  },

  // Stacked bar chart for severity breakdown per time bucket
  stackedBarChart(canvasId, buckets, options = {}) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width = canvas.offsetWidth * 2;
    const h = canvas.height = canvas.offsetHeight * 2;
    ctx.scale(2, 2);

    const cw = canvas.offsetWidth;
    const ch = canvas.offsetHeight;
    const pad = { top: 10, right: 10, bottom: 30, left: 40 };

    if (!buckets.length) return;

    const severityColors = {
      critical: '#ef4444',
      high: '#f97316',
      moderate: '#f59e0b',
      low: '#10b981'
    };
    const severityOrder = ['critical', 'high', 'moderate', 'low'];

    const max = Math.max(...buckets.map(b => {
      return severityOrder.reduce((sum, s) => sum + (b.segments[s] || 0), 0);
    }), 1);

    const barW = (cw - pad.left - pad.right) / buckets.length - 4;

    // Y axis grid
    const borderColor = getComputedStyle(document.documentElement).getPropertyValue('--border').trim();
    const mutedColor = getComputedStyle(document.documentElement).getPropertyValue('--text-muted').trim();
    ctx.strokeStyle = borderColor;
    ctx.lineWidth = 0.5;
    for (let i = 0; i <= 4; i++) {
      const y = pad.top + (ch - pad.top - pad.bottom) * (1 - i / 4);
      ctx.beginPath();
      ctx.moveTo(pad.left, y);
      ctx.lineTo(cw - pad.right, y);
      ctx.stroke();
      ctx.fillStyle = mutedColor;
      ctx.font = '10px Inter, sans-serif';
      ctx.textAlign = 'right';
      ctx.fillText(Math.round(max * i / 4), pad.left - 6, y + 3);
    }

    // Stacked bars
    buckets.forEach((b, i) => {
      const x = pad.left + i * ((cw - pad.left - pad.right) / buckets.length) + 2;
      let yOffset = 0;
      // Draw segments bottom-up: low → moderate → high → critical
      for (let s = severityOrder.length - 1; s >= 0; s--) {
        const sev = severityOrder[s];
        const count = b.segments[sev] || 0;
        if (count === 0) continue;
        const segH = (count / max) * (ch - pad.top - pad.bottom);
        const y = ch - pad.bottom - yOffset - segH;
        ctx.fillStyle = severityColors[sev];
        ctx.beginPath();
        if (yOffset === 0) {
          ctx.roundRect(x, y, barW, segH, [3, 3, 0, 0]);
        } else {
          ctx.rect(x, y, barW, segH);
        }
        ctx.fill();
        yOffset += segH;
      }

      // Count label on top of bar
      const total = severityOrder.reduce((sum, sev) => sum + (b.segments[sev] || 0), 0);
      if (total > 0) {
        const topY = ch - pad.bottom - yOffset;
        ctx.fillStyle = mutedColor;
        ctx.font = 'bold 9px Inter, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(total, x + barW / 2, topY - 3);
      }

      // X label
      if (b.label) {
        ctx.fillStyle = mutedColor;
        ctx.font = '10px Inter, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(b.label, x + barW / 2, ch - pad.bottom + 14);
      }
    });

    // Legend
    const legendY = pad.top;
    let legendX = cw - pad.right;
    ctx.font = '9px Inter, sans-serif';
    ctx.textAlign = 'right';
    for (const sev of severityOrder) {
      const label = sev.charAt(0).toUpperCase() + sev.slice(1);
      const tw = ctx.measureText(label).width;
      legendX -= tw + 4;
      ctx.fillStyle = mutedColor;
      ctx.fillText(label, legendX + tw, legendY + 7);
      legendX -= 10;
      ctx.fillStyle = severityColors[sev];
      ctx.beginPath();
      ctx.arc(legendX + 4, legendY + 4, 3.5, 0, Math.PI * 2);
      ctx.fill();
      legendX -= 8;
    }

    this._trackRender('stackedBarChart', [canvasId, buckets, options]);
  },

  // Risk gauge (semi-circle)
  gauge(canvasId, score) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width = canvas.offsetWidth * 2;
    const h = canvas.height = canvas.offsetHeight * 2;
    ctx.scale(2, 2);

    const cw = canvas.offsetWidth;
    const ch = canvas.offsetHeight;
    const cx = cw / 2;
    const cy = ch - 10;
    const r = Math.min(cx, ch) - 15;
    const lineWidth = 14;

    // Background arc
    ctx.beginPath();
    ctx.arc(cx, cy, r, Math.PI, 0);
    ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--border').trim();
    ctx.lineWidth = lineWidth;
    ctx.stroke();

    // Color segments
    const segments = [
      { end: 0.3, color: '#10b981' },
      { end: 0.6, color: '#f59e0b' },
      { end: 0.8, color: '#f97316' },
      { end: 1.0, color: '#ef4444' }
    ];

    let start = Math.PI;
    for (const seg of segments) {
      const end = Math.PI + Math.PI * seg.end;
      ctx.beginPath();
      ctx.arc(cx, cy, r, start, Math.min(end, Math.PI + Math.PI * (score / 100)));
      ctx.strokeStyle = seg.color;
      ctx.lineWidth = lineWidth;
      ctx.lineCap = 'round';
      ctx.stroke();
      start = end;
      if (score / 100 <= seg.end) break;
    }

    // Score text
    ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--text-primary').trim();
    ctx.font = `bold ${r * 0.45}px Inter, sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'bottom';
    ctx.fillText(score, cx, cy - 5);

    ctx.font = `${r * 0.15}px Inter, sans-serif`;
    ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--text-muted').trim();
    ctx.fillText('RISK SCORE', cx, cy + 12);
    this._trackRender('gauge', [canvasId, score]);
  }
};

// Re-render all charts when theme changes
window.addEventListener('themechange', () => {
  // Small delay to allow CSS variables to update
  setTimeout(() => Charts.reRenderAll(), 50);
});
