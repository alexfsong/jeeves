const Knowledge = {
  currentFilter: { topic: '', type: '' },

  async render(container) {
    // Search bar
    const searchCard = U.el('div', { className: 'card' });
    const searchRow = U.el('div', { className: 'research-input' });

    const searchInput = U.el('input', {
      type: 'text',
      placeholder: 'Search knowledge base...',
      id: 'knowledge-search',
    });
    searchInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') this.search();
    });

    const typeSel = U.el('select', { id: 'knowledge-type-filter' });
    typeSel.innerHTML = '<option value="">All types</option><option value="finding">finding</option><option value="synthesis">synthesis</option><option value="source">source</option><option value="note">note</option>';
    typeSel.value = this.currentFilter.type;

    const topicSel = U.topicSelector(this.currentFilter.topic);
    topicSel.id = 'knowledge-topic-filter';

    const searchBtn = U.el('button', { className: 'btn', textContent: 'Search', onClick: () => this.search() });

    searchRow.appendChild(searchInput);
    searchRow.appendChild(typeSel);
    searchRow.appendChild(topicSel);
    searchRow.appendChild(searchBtn);
    searchCard.appendChild(searchRow);
    container.appendChild(searchCard);

    // Results
    const resultsEl = U.el('div', { id: 'knowledge-results' });
    container.appendChild(resultsEl);

    // Detail panel
    const detailEl = U.el('div', { id: 'knowledge-detail', className: 'hidden' });
    container.appendChild(detailEl);

    // Load initial data
    await this.loadAll();
    setTimeout(() => searchInput.focus(), 50);
  },

  async loadAll() {
    let data;
    if (this.currentFilter.topic) {
      data = await API.get(`/api/topics/${this.currentFilter.topic}/knowledge`);
    } else {
      data = await API.get('/api/knowledge?limit=100');
    }
    this.renderTable(data || []);
  },

  async search() {
    const query = document.getElementById('knowledge-search')?.value?.trim();
    if (!query) {
      await this.loadAll();
      return;
    }

    const data = await API.get(`/api/knowledge/search?q=${encodeURIComponent(query)}`);
    this.renderTable(data || []);
  },

  filterByTopic(slug) {
    this.currentFilter.topic = slug;
    const sel = document.getElementById('knowledge-topic-filter');
    if (sel) sel.value = slug;
    this.loadAll();
  },

  renderTable(entries) {
    const typeFilter = document.getElementById('knowledge-type-filter')?.value || '';
    if (typeFilter) {
      entries = entries.filter(e => e.type === typeFilter);
    }

    const container = document.getElementById('knowledge-results');
    if (!container) return;
    container.innerHTML = '';

    if (entries.length === 0) {
      container.innerHTML = '<div class="empty-state">No knowledge entries found.</div>';
      return;
    }

    const table = U.el('table');
    table.innerHTML = '<thead><tr><th>Title</th><th>Type</th><th>Score</th><th>Date</th></tr></thead>';
    const tbody = U.el('tbody');

    for (const e of entries) {
      const tr = U.el('tr', {
        style: 'cursor:pointer',
        onClick: () => this.showDetail(e.id),
      });
      tr.innerHTML = `
        <td class="text-cream">${e.title}</td>
        <td>${U.badge(e.type)}</td>
        <td>${U.scoreBar(e.score)}</td>
        <td class="text-slate text-sm">${U.timeAgo(e.created_at)}</td>
      `;
      tbody.appendChild(tr);
    }

    table.appendChild(tbody);
    const card = U.el('div', { className: 'card' }, [
      U.el('div', { className: 'card-header', textContent: `${entries.length} entries` }),
      table,
    ]);
    container.appendChild(card);
  },

  async showDetail(id) {
    const detailEl = document.getElementById('knowledge-detail');
    if (!detailEl) return;

    const entry = await API.get(`/api/knowledge/${id}`);
    if (!entry) return;

    detailEl.className = 'detail-panel';
    detailEl.innerHTML = '';

    const closeBtn = U.el('button', {
      className: 'btn btn-sm btn-ghost',
      textContent: 'Close',
      style: 'float:right',
      onClick: () => { detailEl.className = 'hidden'; detailEl.innerHTML = ''; },
    });

    detailEl.appendChild(closeBtn);
    detailEl.appendChild(U.el('div', { className: 'detail-panel-title', textContent: entry.title }));

    const meta = U.el('div', { className: 'detail-panel-meta' });
    meta.innerHTML = `${U.badge(entry.type)} ${U.scoreBar(entry.score)} <span>${U.timeAgo(entry.created_at)}</span>`;
    detailEl.appendChild(meta);

    if (entry.url) {
      const urlRow = U.el('div', { className: 'mb-8 flex-between' });
      urlRow.appendChild(U.el('a', { className: 'url', href: entry.url, target: '_blank', textContent: entry.url }));
      if (entry.type === 'source') {
        urlRow.appendChild(U.el('button', {
          className: 'btn btn-sm btn-ghost',
          textContent: 'Trust this source',
          onClick: async (e) => {
            const domain = U.extractDomain(entry.url);
            if (domain) {
              await API.post('/api/trusted-sources', { domain, trust_level: 1.5 });
              e.target.textContent = 'Trusted';
              e.target.disabled = true;
            }
          },
        }));
      }
      detailEl.appendChild(urlRow);
    }

    detailEl.appendChild(U.el('div', { className: 'md-content', innerHTML: U.md(entry.content) }));

    detailEl.scrollIntoView({ behavior: 'smooth' });
  },
};
