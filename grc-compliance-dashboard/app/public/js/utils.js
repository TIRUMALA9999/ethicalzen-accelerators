// Utility functions

const Utils = {
  formatDate(iso) {
    if (!iso) return '-';
    const d = new Date(iso);
    return d.toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  },

  formatDateShort(iso) {
    if (!iso) return '-';
    const d = new Date(iso);
    return d.toLocaleString('en-US', { month: 'short', day: 'numeric' });
  },

  timeAgo(iso) {
    if (!iso) return '';
    const ms = Date.now() - new Date(iso).getTime();
    if (ms < 60000) return 'just now';
    if (ms < 3600000) return `${Math.floor(ms / 60000)}m ago`;
    if (ms < 86400000) return `${Math.floor(ms / 3600000)}h ago`;
    return `${Math.floor(ms / 86400000)}d ago`;
  },

  truncate(str, len = 40) {
    if (!str) return '';
    return str.length > len ? str.substring(0, len) + '...' : str;
  },

  riskColor(score) {
    if (score <= 30) return 'var(--risk-low)';
    if (score <= 60) return 'var(--risk-moderate)';
    if (score <= 80) return 'var(--risk-high)';
    return 'var(--risk-critical)';
  },

  riskZone(score) {
    if (score <= 30) return 'low';
    if (score <= 60) return 'moderate';
    if (score <= 80) return 'high';
    return 'critical';
  },

  statusBadge(status) {
    const s = (status || '').toLowerCase();
    if (s === 'blocked' || s === 'block') return '<span class="badge badge-blocked">Blocked</span>';
    if (s === 'allowed' || s === 'allow') return '<span class="badge badge-allowed">Allowed</span>';
    if (s === 'review') return '<span class="badge badge-review">Review</span>';
    return `<span class="badge badge-info">${status || 'Unknown'}</span>`;
  },

  scoreBar(score) {
    const pct = Math.min(100, Math.max(0, score * 100));
    const color = score > 0.7 ? 'var(--error)' : score > 0.3 ? 'var(--warning)' : 'var(--success)';
    return `<div class="violation-score-bar" title="${pct.toFixed(1)}% risk score" style="cursor:pointer"><div class="violation-score-bar-fill" style="width:${pct}%;background:${color}"></div></div>`;
  },

  syntaxHighlight(json) {
    if (typeof json === 'string') {
      try { json = JSON.parse(json); } catch { return json; }
    }
    const str = JSON.stringify(json, null, 2);
    return str.replace(/("(\\u[\da-fA-F]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
      (match) => {
        let cls = 'number';
        if (/^"/.test(match)) {
          cls = /:$/.test(match) ? 'key' : 'string';
        } else if (/true|false/.test(match)) {
          cls = 'boolean';
        } else if (/null/.test(match)) {
          cls = 'null';
        }
        return `<span class="json-${cls}">${match}</span>`;
      }
    );
  },

  toast(message, type = 'info') {
    let container = document.querySelector('.toast-container');
    if (!container) {
      container = document.createElement('div');
      container.className = 'toast-container';
      document.body.appendChild(container);
    }
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => toast.remove(), 4000);
  },

  downloadJson(data, filename) {
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  }
};
