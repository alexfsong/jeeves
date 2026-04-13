const TrustedSources = {
  async render(container) {
    // Add form
    const formCard = U.el('div', { className: 'card' });
    formCard.appendChild(U.el('div', { className: 'card-header', textContent: 'Add Trusted Source' }));

    const form = U.el('div', { className: 'research-input' });
    form.appendChild(U.el('input', { type: 'text', placeholder: 'Domain (e.g. arxiv.org)', id: 'ts-domain' }));

    const trustInput = U.el('div', { className: 'trust-slider' });
    trustInput.appendChild(U.el('span', { className: 'text-sm text-slate', textContent: 'Trust:' }));
    const slider = U.el('input', { type: 'range', min: '0', max: '2', step: '0.1', value: '1.5', id: 'ts-trust' });
    const trustLabel = U.el('span', { className: 'text-sm text-gold', textContent: '1.5x', id: 'ts-trust-label' });
    slider.addEventListener('input', () => { trustLabel.textContent = slider.value + 'x'; });
    trustInput.appendChild(slider);
    trustInput.appendChild(trustLabel);
    form.appendChild(trustInput);

    const topicSel = U.topicSelector('');
    topicSel.id = 'ts-topic';
    form.appendChild(topicSel);

    form.appendChild(U.el('input', { type: 'text', placeholder: 'Notes (optional)', id: 'ts-notes' }));
    form.appendChild(U.el('button', { className: 'btn', textContent: 'Add', onClick: () => this.addSource() }));

    formCard.appendChild(form);
    container.appendChild(formCard);

    // Sources list
    const listEl = U.el('div', { id: 'ts-list' });
    container.appendChild(listEl);

    await this.loadSources();
  },

  async loadSources() {
    const sources = await API.get('/api/trusted-sources');
    const listEl = document.getElementById('ts-list');
    if (!listEl) return;
    listEl.innerHTML = '';

    if (!sources?.length) {
      listEl.innerHTML = '<div class="empty-state">No trusted sources yet. Add domains you trust to boost them in search results.</div>';
      return;
    }

    // Group by global vs topic-specific
    const global = sources.filter(s => !s.topic_id);
    const topicScoped = sources.filter(s => s.topic_id);

    if (global.length) {
      const card = U.el('div', { className: 'card' });
      card.appendChild(U.el('div', { className: 'card-header', textContent: 'Global Sources' }));
      this.renderSourceTable(card, global);
      listEl.appendChild(card);
    }

    if (topicScoped.length) {
      const card = U.el('div', { className: 'card' });
      card.appendChild(U.el('div', { className: 'card-header', textContent: 'Topic-Specific Sources' }));
      this.renderSourceTable(card, topicScoped);
      listEl.appendChild(card);
    }
  },

  renderSourceTable(container, sources) {
    const table = U.el('table');
    table.innerHTML = '<thead><tr><th>Domain</th><th>Trust Level</th><th>Scope</th><th>Notes</th><th></th></tr></thead>';
    const tbody = U.el('tbody');

    for (const s of sources) {
      const tr = U.el('tr');
      const trustPct = Math.round(s.trust_level * 50);
      const trustColor = s.trust_level > 1 ? 'var(--gold)' : s.trust_level < 1 ? 'var(--error-red)' : 'var(--slate)';

      tr.innerHTML = `
        <td class="text-cream">${s.domain}</td>
        <td>
          <span class="score-bar">
            <span class="score-bar-track" style="width:60px">
              <span class="score-bar-fill" style="width:${trustPct}%;background:${trustColor}"></span>
            </span>
            <span class="score-bar-label">${s.trust_level}x</span>
          </span>
        </td>
        <td class="text-slate text-sm">${s.topic_id ? 'Topic #' + s.topic_id : 'Global'}</td>
        <td class="text-slate text-sm">${s.notes || ''}</td>
      `;

      const actionTd = U.el('td');
      actionTd.appendChild(U.el('button', {
        className: 'btn btn-sm btn-danger',
        textContent: 'Remove',
        onClick: async () => {
          await API.del(`/api/trusted-sources/${s.id}`);
          await this.loadSources();
        },
      }));
      tr.appendChild(actionTd);
      tbody.appendChild(tr);
    }

    table.appendChild(tbody);
    container.appendChild(table);
  },

  async addSource() {
    const domain = document.getElementById('ts-domain')?.value?.trim();
    const trustLevel = parseFloat(document.getElementById('ts-trust')?.value || '1.5');
    const topicSlug = document.getElementById('ts-topic')?.value || '';
    const notes = document.getElementById('ts-notes')?.value || '';

    if (!domain) return;

    await API.post('/api/trusted-sources', {
      domain: domain.replace(/^www\./, ''),
      trust_level: trustLevel,
      topic_slug: topicSlug || undefined,
      notes,
    });

    // Clear form
    document.getElementById('ts-domain').value = '';
    document.getElementById('ts-notes').value = '';
    document.getElementById('ts-trust').value = '1.5';
    document.getElementById('ts-trust-label').textContent = '1.5x';

    await this.loadSources();
  },
};
