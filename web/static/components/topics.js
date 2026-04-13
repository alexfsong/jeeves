const Topics = {
  async render(container) {
    const controls = U.el('div', { className: 'card flex-between' });
    controls.appendChild(U.el('div', { className: 'card-header', textContent: 'Topics', style: 'margin-bottom:0' }));
    controls.appendChild(U.el('button', { className: 'btn btn-sm', textContent: '+ New Topic', onClick: () => this.createTopic(container) }));
    container.appendChild(controls);

    // Topic graph
    const graphEl = U.el('div', { id: 'topic-graph-container', style: 'width:100%;height:400px;background:var(--card-bg);border:1px solid var(--border);border-radius:8px;overflow:hidden;margin-bottom:16px' });
    container.appendChild(graphEl);

    // Topic list with branches
    const listEl = U.el('div', { id: 'topic-list' });
    container.appendChild(listEl);

    await this.loadTopics(container);
    await this.drawTopicGraph();
  },

  async loadTopics(container) {
    const topics = await API.get('/api/topics');
    const listEl = document.getElementById('topic-list');
    if (!listEl) return;
    listEl.innerHTML = '';

    if (!topics?.length) {
      listEl.innerHTML = '<div class="empty-state">No topics yet.</div>';
      return;
    }

    for (const t of topics) {
      const card = U.el('div', { className: 'card' });

      // Header with actions
      const header = U.el('div', { className: 'flex-between mb-8' });
      header.appendChild(U.el('div', { className: 'topic-card-name', textContent: t.name }));

      const actions = U.el('div', { className: 'flex gap-8' });
      actions.appendChild(U.el('button', {
        className: 'btn btn-sm',
        textContent: 'Branch',
        onClick: () => this.branchTopic(t.slug),
      }));
      actions.appendChild(U.el('button', {
        className: 'btn btn-sm btn-ghost',
        textContent: 'View Knowledge',
        onClick: () => { App.navigate('knowledge'); Knowledge.filterByTopic(t.slug); },
      }));
      actions.appendChild(U.el('button', {
        className: 'btn btn-sm btn-danger',
        textContent: 'Delete',
        onClick: () => this.deleteTopic(t.slug, container),
      }));
      header.appendChild(actions);
      card.appendChild(header);

      if (t.description) {
        card.appendChild(U.el('div', { className: 'text-slate text-sm mb-8', textContent: t.description }));
      }

      const meta = U.el('div', { className: 'topic-card-meta' });
      meta.innerHTML = `<span>${t.knowledge_count || 0} entries</span><span>${U.timeAgo(t.updated_at)}</span>`;
      card.appendChild(meta);

      // Research stack (ancestors)
      const ancestors = await API.get(`/api/topics/${t.slug}/ancestors`);
      if (ancestors?.length) {
        const breadcrumbs = U.el('div', { className: 'breadcrumbs mt-8' });
        for (let i = ancestors.length - 1; i >= 0; i--) {
          const a = ancestors[i];
          breadcrumbs.appendChild(U.el('a', {
            textContent: a.name,
            onClick: () => { App.navigate('knowledge'); Knowledge.filterByTopic(a.slug); },
          }));
          breadcrumbs.appendChild(U.el('span', { className: 'sep', textContent: ' \u2192 ' }));
        }
        breadcrumbs.appendChild(U.el('span', { className: 'text-gold', textContent: t.name }));
        card.appendChild(breadcrumbs);
      }

      // Branches
      const branches = await API.get(`/api/topics/${t.slug}/branches`);
      if (branches?.length) {
        const branchEl = U.el('div', { className: 'mt-8 text-sm' });
        branchEl.appendChild(U.el('span', { className: 'text-slate', textContent: 'Branches: ' }));
        for (const b of branches) {
          branchEl.appendChild(U.el('a', {
            className: 'text-gold',
            textContent: b.name,
            style: 'cursor:pointer;margin-right:8px',
            onClick: () => { App.navigate('knowledge'); Knowledge.filterByTopic(b.slug); },
          }));
        }
        card.appendChild(branchEl);
      }

      listEl.appendChild(card);
    }
  },

  async drawTopicGraph() {
    if (typeof d3 === 'undefined') return;

    const topics = await API.get('/api/topics');
    const links = await API.get('/api/topic-links');

    if (!topics?.length) return;

    const container = document.getElementById('topic-graph-container');
    if (!container) return;
    container.innerHTML = '';

    const width = container.clientWidth;
    const height = container.clientHeight || 400;

    const kindColors = {
      branched_from: '#C9A84C',
      prerequisite: '#6495ED',
      related: '#8899AA',
    };

    const svg = d3.select(container).append('svg')
      .attr('width', width)
      .attr('height', height);

    const g = svg.append('g');
    svg.call(d3.zoom().scaleExtent([0.3, 3]).on('zoom', (e) => g.attr('transform', e.transform)));

    const nodeData = topics.map(t => ({ ...t }));
    const nodeMap = new Map(nodeData.map(n => [n.id, n]));

    const edgeData = (links || [])
      .filter(l => nodeMap.has(l.from_id) && nodeMap.has(l.to_id))
      .map(l => ({ source: nodeMap.get(l.from_id), target: nodeMap.get(l.to_id), kind: l.kind }));

    const link = g.append('g').selectAll('line').data(edgeData).join('line')
      .attr('stroke', d => kindColors[d.kind] || '#8899AA')
      .attr('stroke-width', 2)
      .attr('stroke-opacity', 0.7);

    const node = g.append('g').selectAll('circle').data(nodeData).join('circle')
      .attr('r', 12)
      .attr('fill', 'var(--gold)')
      .attr('stroke', 'var(--bg)')
      .attr('stroke-width', 2)
      .style('cursor', 'pointer')
      .on('click', (event, d) => { App.navigate('knowledge'); Knowledge.filterByTopic(d.slug); })
      .call(d3.drag()
        .on('start', (event, d) => { if (!event.active) sim.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y; })
        .on('drag', (event, d) => { d.fx = event.x; d.fy = event.y; })
        .on('end', (event, d) => { if (!event.active) sim.alphaTarget(0); d.fx = null; d.fy = null; })
      );

    const label = g.append('g').selectAll('text').data(nodeData).join('text')
      .text(d => d.name)
      .attr('font-size', 11)
      .attr('fill', '#F5F0E8')
      .attr('dx', 16)
      .attr('dy', 4);

    const sim = d3.forceSimulation(nodeData)
      .force('link', d3.forceLink(edgeData).distance(120))
      .force('charge', d3.forceManyBody().strength(-300))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .on('tick', () => {
        link.attr('x1', d => d.source.x).attr('y1', d => d.source.y)
            .attr('x2', d => d.target.x).attr('y2', d => d.target.y);
        node.attr('cx', d => d.x).attr('cy', d => d.y);
        label.attr('x', d => d.x).attr('y', d => d.y);
      });
  },

  async branchTopic(parentSlug) {
    const name = prompt('Branch topic name:');
    if (!name) return;
    const desc = prompt('Description (optional):') || '';
    const result = await API.post(`/api/topics/${parentSlug}/branch`, { name, description: desc });
    if (result) {
      await App.loadTopics();
      App.navigate('topics');
    }
  },

  async createTopic(container) {
    const name = prompt('Topic name:');
    if (!name) return;
    const desc = prompt('Description (optional):') || '';
    await API.post('/api/topics', { name, description: desc });
    await App.loadTopics();
    App.navigate('topics');
  },

  async deleteTopic(slug, container) {
    if (!confirm(`Delete topic "${slug}"? Knowledge entries will be unlinked.`)) return;
    await API.del(`/api/topics/${slug}`);
    await App.loadTopics();
    App.navigate('topics');
  },
};
