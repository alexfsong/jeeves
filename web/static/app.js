// Jeeves Dashboard — Core application
const App = {
  currentTab: 'overview',
  topics: [],

  init() {
    this.bindTabs();
    this.navigate('overview');
    this.loadTopics();

    // Keyboard shortcuts
    document.addEventListener('keydown', (e) => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
      if (e.key === '/') { e.preventDefault(); this.navigate('knowledge'); }
      if (e.key === 'r') { e.preventDefault(); this.navigate('research'); }
      if (e.key === 'g') { e.preventDefault(); this.navigate('graph'); }
    });
  },

  bindTabs() {
    document.querySelectorAll('.nav-tab').forEach(tab => {
      tab.addEventListener('click', () => this.navigate(tab.dataset.tab));
    });
  },

  navigate(tab) {
    this.currentTab = tab;
    document.querySelectorAll('.nav-tab').forEach(t => {
      t.classList.toggle('active', t.dataset.tab === tab);
    });

    const app = document.getElementById('app');
    app.innerHTML = '';

    switch (tab) {
      case 'overview': Overview.render(app); break;
      case 'research': Research.render(app); break;
      case 'knowledge': Knowledge.render(app); break;
      case 'graph': GraphView.render(app); break;
      case 'topics': Topics.render(app); break;
      case 'trusted': TrustedSources.render(app); break;
      case 'history': History.render(app); break;
    }
  },

  async loadTopics() {
    this.topics = await API.get('/api/topics') || [];
  },
};

// API client
const API = {
  async get(url) {
    try {
      const res = await fetch(url);
      if (!res.ok) throw new Error(await res.text());
      return res.json();
    } catch (e) {
      console.error('API error:', e);
      return null;
    }
  },

  async post(url, data) {
    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      if (!res.ok) throw new Error(await res.text());
      return res.json();
    } catch (e) {
      console.error('API error:', e);
      return null;
    }
  },

  async del(url) {
    try {
      const res = await fetch(url, { method: 'DELETE' });
      if (!res.ok) throw new Error(await res.text());
      return res.json();
    } catch (e) {
      console.error('API error:', e);
      return null;
    }
  },
};

// Utility helpers
const U = {
  el(tag, attrs = {}, children = []) {
    const el = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
      if (k === 'className') el.className = v;
      else if (k === 'textContent') el.textContent = v;
      else if (k === 'innerHTML') el.innerHTML = v;
      else if (k.startsWith('on')) el.addEventListener(k.slice(2).toLowerCase(), v);
      else el.setAttribute(k, v);
    }
    for (const c of children) {
      if (typeof c === 'string') el.appendChild(document.createTextNode(c));
      else if (c) el.appendChild(c);
    }
    return el;
  },

  badge(type) {
    return `<span class="badge badge-${type}">${type}</span>`;
  },

  scoreBar(score) {
    const pct = Math.round(score * 100);
    return `<span class="score-bar">
      <span class="score-bar-track"><span class="score-bar-fill" style="width:${pct}%"></span></span>
      <span class="score-bar-label">${score.toFixed(2)}</span>
    </span>`;
  },

  timeAgo(dateStr) {
    const d = new Date(dateStr);
    const now = new Date();
    const diff = Math.floor((now - d) / 1000);
    if (diff < 60) return 'just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    if (diff < 604800) return `${Math.floor(diff / 86400)}d ago`;
    return d.toLocaleDateString();
  },

  md(text) {
    if (typeof marked !== 'undefined' && marked.parse) {
      return marked.parse(text || '');
    }
    // Fallback: return escaped text wrapped in <p>
    return '<p>' + (text || '').replace(/</g, '&lt;').replace(/>/g, '&gt;') + '</p>';
  },

  extractDomain(url) {
    try {
      return new URL(url).hostname.replace(/^www\./, '');
    } catch {
      return '';
    }
  },

  topicSelector(selected) {
    const sel = U.el('select', { className: 'topic-select' });
    sel.appendChild(U.el('option', { value: '', textContent: 'No topic' }));
    for (const t of App.topics || []) {
      const opt = U.el('option', { value: t.slug, textContent: t.name });
      if (t.slug === selected) opt.selected = true;
      sel.appendChild(opt);
    }
    return sel;
  },

  resolutionSelector(selected) {
    const sel = U.el('select', { className: 'resolution-select' });
    for (const r of ['glance', 'brief', 'detailed', 'full']) {
      const opt = U.el('option', { value: r, textContent: r });
      if (r === selected) opt.selected = true;
      sel.appendChild(opt);
    }
    return sel;
  },
};
