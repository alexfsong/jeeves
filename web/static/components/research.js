const Research = {
  prefillQuery: '',
  prefillResolution: 'brief',

  render(container) {
    // Input bar
    const form = U.el('div', { className: 'card' });
    form.appendChild(U.el('div', { className: 'card-header', textContent: 'Research' }));

    const inputRow = U.el('div', { className: 'research-input' });
    const queryInput = U.el('input', {
      type: 'text',
      placeholder: 'What would you like to research, sir?',
      value: this.prefillQuery,
      id: 'research-query',
    });
    queryInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') this.runResearch();
    });

    const resSel = U.resolutionSelector(this.prefillResolution);
    resSel.id = 'research-resolution';

    const topicSel = U.topicSelector('');
    topicSel.id = 'research-topic';

    const btn = U.el('button', { className: 'btn', textContent: 'Research', onClick: () => this.runResearch() });

    inputRow.appendChild(queryInput);
    inputRow.appendChild(resSel);
    inputRow.appendChild(topicSel);
    inputRow.appendChild(btn);
    form.appendChild(inputRow);
    container.appendChild(form);

    // Results area
    const results = U.el('div', { id: 'research-results' });
    container.appendChild(results);

    // Focus input
    setTimeout(() => queryInput.focus(), 50);
  },

  prefill(query, resolution) {
    this.prefillQuery = query;
    this.prefillResolution = resolution || 'brief';
    const q = document.getElementById('research-query');
    const r = document.getElementById('research-resolution');
    if (q) q.value = query;
    if (r) r.value = this.prefillResolution;
  },

  async runResearch() {
    const query = document.getElementById('research-query')?.value?.trim();
    const resolution = document.getElementById('research-resolution')?.value || 'brief';
    const topicSlug = document.getElementById('research-topic')?.value || '';

    if (!query) return;

    const resultsEl = document.getElementById('research-results');
    if (!resultsEl) return;

    resultsEl.innerHTML = '<div class="sse-status">Searching...</div>';

    try {
      const response = await fetch('/api/research', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query, resolution, topic_slug: topicSlug }),
      });

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('event: status')) {
            const dataLine = lines[lines.indexOf(line) + 1];
            if (dataLine?.startsWith('data: ')) {
              const statusEl = resultsEl.querySelector('.sse-status');
              if (statusEl) statusEl.textContent = dataLine.slice(6);
            }
          }
          if (line.startsWith('data: ') && line.length > 10) {
            try {
              const data = JSON.parse(line.slice(6));
              this.renderResult(resultsEl, data);
            } catch {}
          }
        }
      }
    } catch (e) {
      resultsEl.innerHTML = `<div class="card"><div class="text-error">Research failed: ${e.message}</div></div>`;
    }
  },

  renderResult(container, result) {
    container.innerHTML = '';

    // Synthesis
    if (result.synthesis) {
      const synthCard = U.el('div', { className: 'card' });
      synthCard.appendChild(U.el('div', { className: 'card-header', textContent: 'Synthesis' }));
      synthCard.appendChild(U.el('div', { className: 'md-content', innerHTML: U.md(result.synthesis) }));
      container.appendChild(synthCard);
    }

    // Warnings
    if (result.warnings?.length) {
      for (const w of result.warnings) {
        container.appendChild(U.el('div', { className: 'card text-slate', textContent: w }));
      }
    }

    // Sub-queries
    if (result.sub_queries?.length > 1) {
      const sqCard = U.el('div', { className: 'card' });
      sqCard.appendChild(U.el('div', { className: 'card-header', textContent: 'Sub-queries' }));
      const list = U.el('ul', { className: 'text-slate text-sm', style: 'padding-left:16px' });
      for (const sq of result.sub_queries) {
        list.appendChild(U.el('li', { textContent: sq }));
      }
      sqCard.appendChild(list);
      container.appendChild(sqCard);
    }

    // Sources
    if (result.results?.length) {
      const srcCard = U.el('div', { className: 'card' });
      srcCard.appendChild(U.el('div', { className: 'card-header', textContent: `Sources (${result.results.length})` }));

      for (const r of result.results.slice(0, 20)) {
        const item = U.el('div', { className: 'mb-16', style: 'padding-bottom:12px;border-bottom:1px solid var(--border)' });
        const header = U.el('div', { className: 'flex-between' });
        header.appendChild(U.el('div', { className: 'text-cream', textContent: r.title || 'Untitled' }));
        header.appendChild(U.el('span', { innerHTML: U.scoreBar(r.score || 0) }));
        item.appendChild(header);

        if (r.url) {
          const urlRow = U.el('div', { className: 'flex-between mt-8' });
          urlRow.appendChild(U.el('a', { className: 'url', href: r.url, target: '_blank', textContent: r.url }));
          const trustBtn = U.el('button', {
            className: 'btn btn-sm btn-ghost',
            textContent: 'Trust source',
            onClick: async (e) => {
              e.stopPropagation();
              const domain = U.extractDomain(r.url);
              if (domain) {
                await API.post('/api/trusted-sources', { domain, trust_level: 1.5, notes: `Trusted from research: ${result.query}` });
                e.target.textContent = 'Trusted';
                e.target.disabled = true;
              }
            },
          });
          urlRow.appendChild(trustBtn);
          item.appendChild(urlRow);
        }

        if (r.snippet) {
          const snippet = r.snippet.length > 300 ? r.snippet.slice(0, 300) + '...' : r.snippet;
          item.appendChild(U.el('div', { className: 'text-slate text-sm mt-8', textContent: snippet }));
        }

        srcCard.appendChild(item);
      }
      container.appendChild(srcCard);
    }

    // Persisted count
    if (result.persisted > 0) {
      container.appendChild(U.el('div', { className: 'card text-success', textContent: `Very good, sir. ${result.persisted} items persisted to your knowledge base.` }));
    }
  },
};
