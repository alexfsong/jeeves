const Overview = {
  async render(container) {
    const stats = await API.get('/api/stats');
    if (!stats) {
      container.innerHTML = '<div class="empty-state">Unable to load dashboard data.</div>';
      return;
    }

    // Stat cards
    const statsRow = U.el('div', { className: 'grid grid-4 mb-16' }, [
      this.statCard(stats.total_topics, 'Topics'),
      this.statCard(stats.total_entries, 'Knowledge Entries'),
      this.statCard(stats.total_sessions, 'Research Sessions'),
      this.statCard(Object.keys(stats.by_type || {}).length, 'Entry Types'),
    ]);
    container.appendChild(statsRow);

    // Main grid: topics + type breakdown
    const mainGrid = U.el('div', { className: 'grid grid-2 mb-16' });

    // Topics list
    const topicsCard = U.el('div', { className: 'card' });
    topicsCard.appendChild(U.el('div', { className: 'card-header flex-between' }, [
      U.el('span', { textContent: 'Topics' }),
      U.el('button', { className: 'btn btn-sm', textContent: '+ New', onClick: () => this.createTopic() }),
    ]));

    const topics = await API.get('/api/topics');
    if (topics && topics.length > 0) {
      const list = U.el('div');
      for (const t of topics.slice(0, 10)) {
        const item = U.el('div', {
          className: 'topic-card mb-8',
          onClick: () => { App.navigate('knowledge'); Knowledge.filterByTopic(t.slug); },
        }, [
          U.el('div', { className: 'topic-card-name', textContent: t.name }),
          t.description ? U.el('div', { className: 'topic-card-desc', textContent: t.description }) : null,
          U.el('div', { className: 'topic-card-meta' }, [
            U.el('span', { textContent: `${t.knowledge_count || 0} entries` }),
            U.el('span', { textContent: U.timeAgo(t.updated_at) }),
          ]),
        ]);
        list.appendChild(item);
      }
      topicsCard.appendChild(list);
    } else {
      topicsCard.appendChild(U.el('div', { className: 'empty-state text-sm', textContent: 'No topics yet. Create one to start organizing research.' }));
    }
    mainGrid.appendChild(topicsCard);

    // Type breakdown
    const typeCard = U.el('div', { className: 'card' });
    typeCard.appendChild(U.el('div', { className: 'card-header', textContent: 'Knowledge by Type' }));
    const byType = stats.by_type || {};
    if (Object.keys(byType).length > 0) {
      const total = Object.values(byType).reduce((a, b) => a + b, 0);
      for (const [type, count] of Object.entries(byType)) {
        const pct = total > 0 ? Math.round((count / total) * 100) : 0;
        const row = U.el('div', { className: 'flex-between mb-8' }, [
          U.el('span', { innerHTML: U.badge(type) }),
          U.el('span', { className: 'flex gap-8', style: 'align-items:center' }, [
            U.el('span', { className: 'text-sm text-slate', textContent: `${count}` }),
            U.el('span', { className: 'score-bar-track', style: 'width:100px' }, [
              U.el('span', { className: 'score-bar-fill', style: `width:${pct}%` }),
            ]),
          ]),
        ]);
        typeCard.appendChild(row);
      }
    } else {
      typeCard.appendChild(U.el('div', { className: 'empty-state text-sm', textContent: 'No knowledge entries yet.' }));
    }
    mainGrid.appendChild(typeCard);
    container.appendChild(mainGrid);

    // Recent sessions
    const sessCard = U.el('div', { className: 'card' });
    sessCard.appendChild(U.el('div', { className: 'card-header', textContent: 'Recent Research' }));
    const sessions = stats.recent_sessions || [];
    if (sessions.length > 0) {
      const table = U.el('table');
      table.innerHTML = '<thead><tr><th>Query</th><th>Resolution</th><th>Results</th><th>When</th></tr></thead>';
      const tbody = U.el('tbody');
      for (const s of sessions.slice(0, 10)) {
        const tr = U.el('tr', {
          style: 'cursor:pointer',
          onClick: () => { App.navigate('research'); Research.prefill(s.query, s.resolution); },
        });
        tr.innerHTML = `
          <td class="text-cream">${s.query}</td>
          <td><span class="badge badge-source">${s.resolution}</span></td>
          <td>${s.result_count}</td>
          <td class="text-slate text-sm">${U.timeAgo(s.timestamp)}</td>
        `;
        tbody.appendChild(tr);
      }
      table.appendChild(tbody);
      sessCard.appendChild(table);
    } else {
      sessCard.appendChild(U.el('div', { className: 'empty-state text-sm', textContent: 'No research sessions yet. Try the Research tab.' }));
    }
    container.appendChild(sessCard);
  },

  statCard(value, label) {
    return U.el('div', { className: 'stat-card' }, [
      U.el('div', { className: 'stat-value', textContent: String(value || 0) }),
      U.el('div', { className: 'stat-label', textContent: label }),
    ]);
  },

  async createTopic() {
    const name = prompt('Topic name:');
    if (!name) return;
    const desc = prompt('Description (optional):') || '';
    const result = await API.post('/api/topics', { name, description: desc });
    if (result) {
      await App.loadTopics();
      App.navigate('overview');
    }
  },
};
