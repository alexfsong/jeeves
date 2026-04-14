const Setup = {
  async render(container) {
    const status = await API.get('/api/status');
    if (!status) {
      container.innerHTML = '<div class="empty-state">Unable to load system status.</div>';
      return;
    }

    const card = U.el('div', { className: 'card' });
    card.appendChild(U.el('div', { className: 'card-header', textContent: 'System Status' }));

    // Status indicators
    const grid = U.el('div', { className: 'grid grid-3 mb-16' });

    grid.appendChild(this.statusTile(
      'Search',
      status.brave_configured ? 'Brave API ready' : 'No search API key',
      status.brave_configured,
    ));

    grid.appendChild(this.statusTile(
      'Local LLM',
      status.local_available
        ? `${status.local_model || 'Ollama'} connected`
        : 'Ollama not available',
      status.local_available,
    ));

    grid.appendChild(this.statusTile(
      'Cloud LLM',
      status.cloud_available
        ? `${status.cloud_model || 'Anthropic'} ready`
        : 'No API key configured',
      status.cloud_available,
    ));

    card.appendChild(grid);

    // Model selector (if ollama is available)
    if (status.local_available) {
      const modelSection = U.el('div', { className: 'mb-16' });
      modelSection.appendChild(U.el('div', {
        className: 'text-cream mb-8',
        textContent: 'Local Model',
      }));

      const modelRow = U.el('div', { className: 'flex gap-8' });
      const modelSelect = U.el('select', { id: 'model-select' });
      modelSelect.appendChild(U.el('option', { value: '', textContent: 'Loading models...' }));
      modelRow.appendChild(modelSelect);

      const applyBtn = U.el('button', {
        className: 'btn btn-sm',
        textContent: 'Apply',
        onClick: () => this.applyModel(),
      });
      modelRow.appendChild(applyBtn);
      modelSection.appendChild(modelRow);
      card.appendChild(modelSection);

      // Load models async
      this.loadModels(status.local_model);
    }

    // Config info
    const infoSection = U.el('div', { className: 'text-slate text-sm' });
    infoSection.innerHTML = `
      <p class="mb-8">Configure API keys in <code>~/.jeeves/config.toml</code> or via environment variables:</p>
      <code>BRAVE_API_KEY</code> &mdash; Web search<br>
      <code>ANTHROPIC_API_KEY</code> &mdash; Cloud synthesis (detailed/full resolution)<br>
      <code>TAVILY_API_KEY</code> &mdash; Alternative search provider
    `;
    card.appendChild(infoSection);
    container.appendChild(card);

    // Capabilities table
    const capCard = U.el('div', { className: 'card' });
    capCard.appendChild(U.el('div', { className: 'card-header', textContent: 'Resolution Capabilities' }));

    const table = U.el('table');
    table.innerHTML = `
      <thead><tr>
        <th>Resolution</th><th>Search</th><th>LLM</th><th>Persist</th><th>Status</th>
      </tr></thead>
      <tbody>
        <tr>
          <td class="text-cream">Glance</td>
          <td>Snippets only</td><td>None</td><td>No</td>
          <td>${status.brave_configured ? '<span class="text-success">Ready</span>' : '<span class="text-error">Needs search API</span>'}</td>
        </tr>
        <tr>
          <td class="text-cream">Brief</td>
          <td>Top 3 URLs</td><td>Local (Ollama)</td><td>No</td>
          <td>${status.brave_configured && status.local_available ? '<span class="text-success">Ready</span>' : '<span class="text-error">Needs ' + (!status.brave_configured ? 'search API' : 'Ollama') + '</span>'}</td>
        </tr>
        <tr>
          <td class="text-cream">Detailed</td>
          <td>Top 10 URLs</td><td>Cloud (Claude)</td><td>Yes</td>
          <td>${status.brave_configured && status.cloud_available ? '<span class="text-success">Ready</span>' : '<span class="text-error">Needs ' + (!status.brave_configured ? 'search API' : 'Anthropic key') + '</span>'}</td>
        </tr>
        <tr>
          <td class="text-cream">Full</td>
          <td>All URLs (paginated)</td><td>Cloud (Claude)</td><td>Yes</td>
          <td>${status.brave_configured && status.cloud_available ? '<span class="text-success">Ready</span>' : '<span class="text-error">Needs ' + (!status.brave_configured ? 'search API' : 'Anthropic key') + '</span>'}</td>
        </tr>
      </tbody>
    `;
    capCard.appendChild(table);
    container.appendChild(capCard);
  },

  statusTile(label, detail, ok) {
    const tile = U.el('div', { className: 'stat-card' });
    tile.appendChild(U.el('div', {
      className: ok ? 'stat-value text-success' : 'stat-value text-error',
      style: 'font-size:20px',
      textContent: ok ? 'Ready' : 'Offline',
    }));
    tile.appendChild(U.el('div', { className: 'stat-label', textContent: label }));
    tile.appendChild(U.el('div', {
      className: 'text-sm mt-8 ' + (ok ? 'text-slate' : 'text-error'),
      textContent: detail,
    }));
    return tile;
  },

  async loadModels(currentModel) {
    const data = await API.get('/api/ollama/models');
    const select = document.getElementById('model-select');
    if (!select || !data) return;

    select.innerHTML = '';
    const models = data.models || [];
    if (models.length === 0) {
      select.appendChild(U.el('option', { value: '', textContent: 'No models installed' }));
      return;
    }
    for (const m of models) {
      const opt = U.el('option', { value: m, textContent: m });
      if (m === currentModel) opt.selected = true;
      select.appendChild(opt);
    }
  },

  async applyModel() {
    const select = document.getElementById('model-select');
    if (!select || !select.value) return;

    const result = await API.put('/api/config/model', { model: select.value });
    if (result && result.status === 'ok') {
      select.style.borderColor = 'var(--success-green)';
      setTimeout(() => { select.style.borderColor = ''; }, 2000);
    }
  },
};
