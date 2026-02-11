// Main App — SPA Router + View Loader

const App = {
  currentView: null,
  connectionStatus: 'offline',

  async init() {
    Theme.init();
    this.bindEvents();
    this.checkConnection();
    setInterval(() => this.checkConnection(), 30000);

    // Route on hash change
    window.addEventListener('hashchange', () => this.route());
    this.route();
  },

  bindEvents() {
    document.getElementById('theme-toggle')?.addEventListener('click', () => Theme.toggle());
  },

  async checkConnection() {
    try {
      const health = await API.health();
      this.connectionStatus = health.cloud?.connected ? 'live' : 'cached';
    } catch {
      this.connectionStatus = 'offline';
    }
    this.updateConnectionBadge();
  },

  updateConnectionBadge() {
    const badge = document.getElementById('connection-badge');
    if (!badge) return;
    const labels = { live: 'Live', cached: 'Cached', offline: 'Offline' };
    badge.className = `connection-badge ${this.connectionStatus}`;
    badge.innerHTML = `<span class="connection-dot"></span>${labels[this.connectionStatus]}`;
  },

  async route() {
    const hash = window.location.hash.slice(1) || 'dashboard';
    const viewName = hash.split('?')[0];

    // Update active nav
    document.querySelectorAll('.nav-item').forEach(el => {
      el.classList.toggle('active', el.getAttribute('data-view') === viewName);
    });

    // Update header title
    const titles = {
      dashboard: 'Executive Dashboard',
      violations: 'Violations',
      compliance: 'Compliance Matrix',
      exports: 'Export Builder',
      taxii: 'TAXII Browser',
      risk: 'Risk Overview',
      evidence: 'Evidence Trail',
      drift: 'Drift Alerts',
      settings: 'Settings'
    };
    const headerTitle = document.getElementById('header-title');
    if (headerTitle) headerTitle.textContent = titles[viewName] || viewName;

    // Load view
    await this.loadView(viewName);
  },

  async loadView(name) {
    const container = document.getElementById('view-container');
    if (!container) return;

    // Cleanup previous view (e.g., close SSE connections)
    if (typeof window._viewCleanup === 'function') {
      window._viewCleanup();
      window._viewCleanup = null;
    }

    try {
      const res = await fetch(`/views/${name}.html`);
      if (!res.ok) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-icon">404</div><div class="empty-state-title">View not found</div></div>`;
        return;
      }
      container.innerHTML = await res.text();

      // innerHTML does NOT execute <script> tags — re-create them so the browser runs them
      container.querySelectorAll('script').forEach(oldScript => {
        const newScript = document.createElement('script');
        newScript.textContent = oldScript.textContent;
        oldScript.parentNode.replaceChild(newScript, oldScript);
      });

      // Execute view init if defined
      if (window[`init_${name}`]) {
        await window[`init_${name}`]();
      }
    } catch (err) {
      container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error loading view</div><p>${err.message}</p></div>`;
    }
  }
};

// Boot
document.addEventListener('DOMContentLoaded', () => App.init());
