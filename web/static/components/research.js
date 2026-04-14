const Research = {
  prefillQuery: '',
  prefillResolution: '',
  lastResult: null,
  researchHistory: [], // stack of queries in this session

  render(container) {
    // Determine best available resolution
    const bestRes = this.bestResolution();

    // Search bar — prominent, centered
    const hero = U.el('div', { className: 'research-hero' });
    const form = U.el('div', { className: 'card research-card' });

    const queryInput = U.el('input', {
      type: 'text',
      placeholder: 'What would you like to research?',
      value: this.prefillQuery,
      id: 'research-query',
      className: 'research-query-input',
    });
    queryInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) this.startResearch();
    });
    form.appendChild(queryInput);

    // Controls row — resolution + options
    const controls = U.el('div', { className: 'research-controls' });

    // Resolution selector with descriptions
    const resGroup = U.el('div', { className: 'resolution-group' });
    const resolutions = [
      { value: 'glance', label: 'Glance', desc: 'Quick snippets' },
      { value: 'brief', label: 'Brief', desc: 'Summarized' },
      { value: 'detailed', label: 'Detailed', desc: 'Full synthesis' },
      { value: 'full', label: 'Deep', desc: 'Exhaustive' },
    ];

    for (const r of resolutions) {
      const selected = (this.prefillResolution || bestRes) === r.value;
      const btn = U.el('button', {
        className: 'res-btn' + (selected ? ' res-btn-active' : ''),
        'data-res': r.value,
        onClick: () => this.selectResolution(r.value),
      });
      btn.appendChild(U.el('span', { className: 'res-btn-label', textContent: r.label }));
      btn.appendChild(U.el('span', { className: 'res-btn-desc', textContent: r.desc }));
      resGroup.appendChild(btn);
    }
    controls.appendChild(resGroup);

    const rightControls = U.el('div', { className: 'flex gap-8' });
    const topicSel = U.topicSelector('');
    topicSel.id = 'research-topic';
    rightControls.appendChild(topicSel);

    const goBtn = U.el('button', {
      className: 'btn research-go-btn',
      textContent: 'Research',
      onClick: () => this.startResearch(),
    });
    rightControls.appendChild(goBtn);
    controls.appendChild(rightControls);

    form.appendChild(controls);
    hero.appendChild(form);
    container.appendChild(hero);

    // Research trail — show breadcrumbs of this session's queries
    if (this.researchHistory.length > 0) {
      const trail = U.el('div', { className: 'research-trail card' });
      trail.appendChild(U.el('div', { className: 'text-sm text-slate mb-8', textContent: 'Research trail' }));
      const crumbs = U.el('div', { className: 'breadcrumbs' });
      for (let i = 0; i < this.researchHistory.length; i++) {
        if (i > 0) crumbs.appendChild(U.el('span', { className: 'sep', textContent: ' \u2192 ' }));
        const q = this.researchHistory[i];
        const link = U.el('a', {
          textContent: q.length > 40 ? q.slice(0, 40) + '...' : q,
          onClick: () => this.prefill(q),
        });
        crumbs.appendChild(link);
      }
      trail.appendChild(crumbs);
      container.appendChild(trail);
    }

    // Results area
    const results = U.el('div', { id: 'research-results' });
    container.appendChild(results);

    // If we have a last result, re-render it
    if (this.lastResult) {
      this.renderResult(results, this.lastResult);
    }

    setTimeout(() => queryInput.focus(), 50);
  },

  bestResolution() {
    const s = App.systemStatus;
    if (!s) return 'glance';
    if (s.cloud_available && s.brave_configured) return 'detailed';
    if (s.local_available && s.brave_configured) return 'brief';
    return 'glance';
  },

  selectResolution(value) {
    this.prefillResolution = value;
    document.querySelectorAll('.res-btn').forEach(btn => {
      btn.classList.toggle('res-btn-active', btn.dataset.res === value);
    });
  },

  prefill(query, resolution) {
    this.prefillQuery = query;
    if (resolution) this.prefillResolution = resolution;
    const q = document.getElementById('research-query');
    if (q) {
      q.value = query;
      q.focus();
    }
  },

  async startResearch() {
    const query = document.getElementById('research-query')?.value?.trim();
    const resolution = this.prefillResolution || this.bestResolution();
    const topicSlug = document.getElementById('research-topic')?.value || '';

    if (!query) return;

    // Track in history
    if (!this.researchHistory.includes(query)) {
      this.researchHistory.push(query);
    }

    const resultsEl = document.getElementById('research-results');
    if (!resultsEl) return;

    // Show progress
    resultsEl.innerHTML = '';
    const progressCard = U.el('div', { className: 'card research-progress' });
    const spinner = U.el('div', { className: 'research-spinner' });
    spinner.appendChild(U.el('div', { className: 'spinner-ring' }));
    spinner.appendChild(U.el('div', { id: 'research-status-text', className: 'text-slate', textContent: 'Searching...' }));
    spinner.appendChild(U.el('div', { id: 'research-stage-pill', className: 'stage-pill', textContent: '' }));
    progressCard.appendChild(spinner);
    resultsEl.appendChild(progressCard);

    try {
      // Check prior knowledge first
      const prior = await API.get(`/api/knowledge/prior?q=${encodeURIComponent(query)}`);
      if (prior && prior.count > 0) {
        const priorCard = U.el('div', { className: 'card prior-knowledge-card' });
        priorCard.appendChild(U.el('div', { className: 'card-header', textContent: `You already have ${prior.count} related entries` }));
        const list = U.el('div', { className: 'text-sm' });
        for (const e of prior.entries.slice(0, 5)) {
          const item = U.el('div', { className: 'flex-between mb-8' });
          item.appendChild(U.el('span', { innerHTML: `${U.badge(e.type)} <span class="text-cream">${e.title}</span>` }));
          item.appendChild(U.el('a', {
            className: 'text-gold text-sm',
            textContent: 'view',
            style: 'cursor:pointer',
            onClick: () => { App.navigate('knowledge'); Knowledge.showDetail(e.id); },
          }));
          list.appendChild(item);
        }
        priorCard.appendChild(list);
        resultsEl.insertBefore(priorCard, resultsEl.firstChild.nextSibling || null);
      }

      // Use auto-topic endpoint if no topic selected and resolution is detailed+
      const useAutoTopic = !topicSlug && (resolution === 'detailed' || resolution === 'full');
      const endpoint = useAutoTopic ? '/api/research/with-topic' : '/api/research';
      const payload = useAutoTopic
        ? { query, resolution }
        : { query, resolution, topic_slug: topicSlug };

      const response = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      // Parse SSE event blocks. Each event is "event: NAME\ndata: PAYLOAD\n\n".
      const handleEvent = (event, data) => {
        if (event === 'status') {
          const statusEl = document.getElementById('research-status-text');
          if (statusEl) statusEl.textContent = data;
        } else if (event === 'progress') {
          try {
            const ev = JSON.parse(data);
            const pill = document.getElementById('research-stage-pill');
            if (pill) {
              let label = ev.stage;
              if (ev.detail) label += ` · ${ev.detail}`;
              if (ev.stage === 'verify' && ev.detail === 'complete') {
                label = `verified · ${ev.tool_calls || 0} checks · ${ev.new_sources || 0} new`;
              }
              pill.textContent = label;
            }
          } catch {}
        } else if (event === 'result') {
          try {
            const parsed = JSON.parse(data);
            this.lastResult = parsed;
            this.renderResult(resultsEl, parsed);
          } catch {}
        } else if (event === 'error') {
          resultsEl.innerHTML = `<div class="card"><div class="text-error">Research failed: ${data}</div></div>`;
        }
      };

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        // Split on blank lines to extract full event blocks.
        let sep;
        while ((sep = buffer.indexOf('\n\n')) !== -1) {
          const block = buffer.slice(0, sep);
          buffer = buffer.slice(sep + 2);

          let eventName = 'message';
          let dataParts = [];
          for (const line of block.split('\n')) {
            if (line.startsWith('event: ')) {
              eventName = line.slice(7).trim();
            } else if (line.startsWith('data: ')) {
              dataParts.push(line.slice(6));
            }
          }
          handleEvent(eventName, dataParts.join('\n'));
        }
      }
    } catch (e) {
      resultsEl.innerHTML = `<div class="card"><div class="text-error">Research failed: ${e.message}</div></div>`;
    }
  },

  renderResult(container, result) {
    container.innerHTML = '';

    // Warnings at the top (but styled less alarming)
    if (result.warnings?.length) {
      const warnCard = U.el('div', { className: 'card research-warnings' });
      for (const w of result.warnings) {
        warnCard.appendChild(U.el('div', { className: 'text-slate text-sm', textContent: w }));
      }
      container.appendChild(warnCard);
    }

    // Synthesis — the main event
    if (result.synthesis) {
      const synthCard = U.el('div', { className: 'card synthesis-card' });
      const header = U.el('div', { className: 'card-header flex-between' });
      header.appendChild(U.el('span', { textContent: 'Synthesis' }));

      // Deepen button for non-detailed results
      if (result.resolution === 'glance' || result.resolution === 'brief') {
        const deepenBtn = U.el('button', {
          className: 'btn btn-sm',
          textContent: 'Go deeper',
          onClick: () => {
            const nextRes = result.resolution === 'glance' ? 'brief' : 'detailed';
            this.selectResolution(nextRes);
            this.startResearch();
          },
        });
        header.appendChild(deepenBtn);
      }

      synthCard.appendChild(header);

      if (result.verification && !result.verification.skipped) {
        const v = result.verification;
        const badge = U.el('div', {
          className: 'verified-badge',
          textContent: `\u2713 Verified · ${v.tool_calls || 0} checks · ${v.new_sources || 0} new sources`,
        });
        synthCard.appendChild(badge);
      }

      synthCard.appendChild(U.el('div', { className: 'md-content', innerHTML: U.md(result.synthesis) }));
      container.appendChild(synthCard);
    } else if (!result.warnings?.length) {
      // No synthesis and no warnings — show snippets nicely
      const noSynthCard = U.el('div', { className: 'card' });
      noSynthCard.appendChild(U.el('div', { className: 'card-header flex-between' }, [
        U.el('span', { textContent: 'Search Results' }),
        U.el('button', {
          className: 'btn btn-sm',
          textContent: 'Synthesize with AI',
          onClick: () => {
            this.selectResolution('detailed');
            this.startResearch();
          },
        }),
      ]));
      container.appendChild(noSynthCard);
    }

    // Persisted indicator
    if (result.persisted > 0) {
      const persistCard = U.el('div', { className: 'card persist-indicator' });
      const msg = U.el('div', { className: 'flex gap-8', style: 'align-items:center' });
      msg.appendChild(U.el('span', { className: 'text-success', textContent: `\u2713 ${result.persisted} items saved to knowledge base` }));
      if (result.topic_slug) {
        msg.appendChild(U.el('a', {
          className: 'text-gold text-sm',
          textContent: `View topic: ${result.topic_name || result.topic_slug}`,
          style: 'cursor:pointer',
          onClick: () => { App.navigate('knowledge'); Knowledge.filterByTopic(result.topic_slug); },
        }));
      }
      persistCard.appendChild(msg);
      container.appendChild(persistCard);
    }

    // Follow-up suggestions — the key UX differentiator
    if (result.synthesis) {
      const followUpCard = U.el('div', { className: 'card follow-up-card', id: 'follow-ups' });
      followUpCard.appendChild(U.el('div', { className: 'card-header', textContent: 'Dig deeper' }));
      followUpCard.appendChild(U.el('div', {
        className: 'text-slate text-sm',
        id: 'follow-up-loading',
        textContent: 'Generating follow-up questions...',
      }));
      container.appendChild(followUpCard);
      this.loadFollowUps(result.query, result.synthesis, result.topic_slug);
    }

    // Sources — collapsible
    if (result.results?.length) {
      const srcCard = U.el('div', { className: 'card' });
      const srcHeader = U.el('div', {
        className: 'card-header sources-toggle',
        style: 'cursor:pointer',
        onClick: () => {
          const list = document.getElementById('sources-list');
          if (list) list.classList.toggle('hidden');
        },
      });
      srcHeader.appendChild(U.el('span', { textContent: `Sources (${result.results.length})` }));
      srcHeader.appendChild(U.el('span', { className: 'text-slate text-sm', textContent: 'click to expand' }));
      srcCard.appendChild(srcHeader);

      const sourcesList = U.el('div', { id: 'sources-list', className: result.synthesis ? 'hidden' : '' });
      for (const r of result.results.slice(0, 20)) {
        const item = U.el('div', { className: 'source-item' });
        const header = U.el('div', { className: 'flex-between' });
        const titleRow = U.el('div', { className: 'flex gap-8', style: 'align-items:center' });
        titleRow.appendChild(U.el('div', { className: 'text-cream', textContent: r.title || 'Untitled' }));
        if (r.verified) {
          titleRow.appendChild(U.el('span', { className: 'verified-chip', textContent: 'Verified' }));
        }
        header.appendChild(titleRow);
        header.appendChild(U.el('span', { innerHTML: U.scoreBar(r.score || 0) }));
        item.appendChild(header);

        if (r.url) {
          const urlRow = U.el('div', { className: 'flex-between mt-8' });
          urlRow.appendChild(U.el('a', { className: 'url', href: r.url, target: '_blank', textContent: r.url }));
          const trustBtn = U.el('button', {
            className: 'btn btn-sm btn-ghost',
            textContent: 'Trust',
            onClick: async (e) => {
              e.stopPropagation();
              const domain = U.extractDomain(r.url);
              if (domain) {
                await API.post('/api/trusted-sources', {
                  domain,
                  trust_level: 1.5,
                  topic_slug: result.topic_slug || '',
                  notes: `Trusted from research: ${result.query}`,
                });
                e.target.textContent = 'Trusted';
                e.target.disabled = true;
                e.target.className = 'btn btn-sm btn-ghost text-success';
              }
            },
          });
          urlRow.appendChild(trustBtn);
          item.appendChild(urlRow);
        }

        if (r.snippet) {
          const snippet = r.snippet.length > 200 ? r.snippet.slice(0, 200) + '...' : r.snippet;
          item.appendChild(U.el('div', { className: 'text-slate text-sm mt-8', textContent: snippet }));
        }
        sourcesList.appendChild(item);
      }
      srcCard.appendChild(sourcesList);
      container.appendChild(srcCard);
    }

    // Sub-queries (collapsed by default)
    if (result.sub_queries?.length > 1) {
      const sqCard = U.el('div', { className: 'card' });
      sqCard.appendChild(U.el('div', {
        className: 'card-header text-slate text-sm',
        style: 'cursor:pointer',
        textContent: `Sub-queries (${result.sub_queries.length})`,
        onClick: (e) => {
          const list = e.target.nextElementSibling;
          if (list) list.classList.toggle('hidden');
        },
      }));
      const list = U.el('ul', { className: 'text-slate text-sm hidden', style: 'padding-left:16px' });
      for (const sq of result.sub_queries) {
        list.appendChild(U.el('li', { textContent: sq }));
      }
      sqCard.appendChild(list);
      container.appendChild(sqCard);
    }
  },

  async loadFollowUps(query, synthesis, topicSlug) {
    const data = await API.post('/api/research/follow-ups', {
      query,
      synthesis: synthesis.slice(0, 3000), // Don't send too much
      topic_slug: topicSlug || '',
    });

    const card = document.getElementById('follow-ups');
    const loading = document.getElementById('follow-up-loading');
    if (!card) return;

    if (loading) loading.remove();

    const followUps = data?.follow_ups || [];
    if (followUps.length === 0) {
      card.appendChild(U.el('div', { className: 'text-slate text-sm', textContent: 'No follow-up suggestions available.' }));
      return;
    }

    const chips = U.el('div', { className: 'follow-up-chips' });
    for (const q of followUps) {
      const chip = U.el('button', {
        className: 'follow-up-chip',
        textContent: q,
        onClick: () => {
          this.prefill(q);
          this.startResearch();
        },
      });
      chips.appendChild(chip);
    }
    card.appendChild(chips);
  },
};
