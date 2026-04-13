const GraphView = {
  simulation: null,

  async render(container) {
    // Controls
    const controls = U.el('div', { className: 'card flex gap-8', style: 'align-items:center' });
    controls.appendChild(U.el('span', { className: 'text-slate', textContent: 'Filter by topic:' }));
    const topicSel = U.topicSelector('');
    topicSel.id = 'graph-topic';
    topicSel.addEventListener('change', () => this.loadGraph());
    controls.appendChild(topicSel);

    const legend = U.el('div', { className: 'flex gap-16', style: 'margin-left:auto' });
    legend.innerHTML = `
      <span class="text-sm"><span style="color:#6495ED">\u25cf</span> finding</span>
      <span class="text-sm"><span style="color:var(--gold)">\u25cf</span> synthesis</span>
      <span class="text-sm"><span style="color:var(--slate)">\u25cf</span> source</span>
      <span class="text-sm"><span style="color:var(--success-green)">\u25cf</span> note</span>
    `;
    controls.appendChild(legend);
    container.appendChild(controls);

    // Graph container
    const graphEl = U.el('div', { id: 'graph-container' });
    container.appendChild(graphEl);

    // Detail sidebar
    const detail = U.el('div', { id: 'graph-detail', className: 'hidden' });
    container.appendChild(detail);

    await this.loadGraph();
  },

  async loadGraph() {
    if (typeof d3 === 'undefined') {
      document.getElementById('graph-container').innerHTML = '<div class="empty-state">D3.js not loaded. Vendor file missing.</div>';
      return;
    }

    const topicSlug = document.getElementById('graph-topic')?.value || '';
    const url = topicSlug ? `/api/knowledge/graph?topic=${topicSlug}` : '/api/knowledge/graph';
    const data = await API.get(url);

    if (!data || !data.nodes?.length) {
      document.getElementById('graph-container').innerHTML = '<div class="empty-state">No knowledge entries to graph yet.</div>';
      return;
    }

    this.drawGraph(data);
  },

  drawGraph(data) {
    const container = document.getElementById('graph-container');
    container.innerHTML = '';

    const width = container.clientWidth;
    const height = container.clientHeight || 600;

    const colorMap = {
      finding: '#6495ED',
      synthesis: '#C9A84C',
      source: '#8899AA',
      note: '#66CC99',
    };

    const edgeStyleMap = {
      related: { stroke: '#8899AA', dasharray: '4,4' },
      subtopic: { stroke: '#8899AA', dasharray: '' },
      contradicts: { stroke: '#CC6666', dasharray: '6,3' },
      supersedes: { stroke: '#C9A84C', dasharray: '' },
    };

    const svg = d3.select(container).append('svg')
      .attr('width', width)
      .attr('height', height);

    const g = svg.append('g');

    // Zoom
    const zoom = d3.zoom()
      .scaleExtent([0.2, 4])
      .on('zoom', (event) => g.attr('transform', event.transform));
    svg.call(zoom);

    // Links
    const link = g.append('g')
      .selectAll('line')
      .data(data.edges || [])
      .join('line')
      .attr('stroke', d => (edgeStyleMap[d.kind] || edgeStyleMap.related).stroke)
      .attr('stroke-dasharray', d => (edgeStyleMap[d.kind] || edgeStyleMap.related).dasharray)
      .attr('stroke-width', 1.5)
      .attr('stroke-opacity', 0.6);

    // Nodes
    const nodeData = data.nodes.map(n => ({ ...n }));
    const edgeData = (data.edges || []).map(e => ({
      source: nodeData.find(n => n.id === e.from_id),
      target: nodeData.find(n => n.id === e.to_id),
      kind: e.kind,
    })).filter(e => e.source && e.target);

    const node = g.append('g')
      .selectAll('circle')
      .data(nodeData)
      .join('circle')
      .attr('r', d => 5 + Math.min(d.score * 10, 10))
      .attr('fill', d => colorMap[d.type] || '#8899AA')
      .attr('stroke', '#0F1A2E')
      .attr('stroke-width', 1.5)
      .style('cursor', 'pointer')
      .on('click', (event, d) => this.showNodeDetail(d))
      .call(d3.drag()
        .on('start', (event, d) => {
          if (!event.active) this.simulation.alphaTarget(0.3).restart();
          d.fx = d.x; d.fy = d.y;
        })
        .on('drag', (event, d) => { d.fx = event.x; d.fy = event.y; })
        .on('end', (event, d) => {
          if (!event.active) this.simulation.alphaTarget(0);
          d.fx = null; d.fy = null;
        })
      );

    // Labels
    const label = g.append('g')
      .selectAll('text')
      .data(nodeData)
      .join('text')
      .text(d => d.title.length > 30 ? d.title.slice(0, 30) + '...' : d.title)
      .attr('font-size', 10)
      .attr('fill', '#D0D0D0')
      .attr('dx', 12)
      .attr('dy', 4);

    // Simulation
    this.simulation = d3.forceSimulation(nodeData)
      .force('link', d3.forceLink(edgeData).id(d => d.id).distance(80))
      .force('charge', d3.forceManyBody().strength(-200))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collision', d3.forceCollide().radius(20))
      .on('tick', () => {
        link
          .attr('x1', d => d.source.x)
          .attr('y1', d => d.source.y)
          .attr('x2', d => d.target.x)
          .attr('y2', d => d.target.y);
        node
          .attr('cx', d => d.x)
          .attr('cy', d => d.y);
        label
          .attr('x', d => d.x)
          .attr('y', d => d.y);
      });
  },

  async showNodeDetail(node) {
    const detailEl = document.getElementById('graph-detail');
    if (!detailEl) return;

    const entry = await API.get(`/api/knowledge/${node.id}`);
    if (!entry) return;

    detailEl.className = 'detail-panel';
    detailEl.innerHTML = '';
    detailEl.appendChild(U.el('button', {
      className: 'btn btn-sm btn-ghost',
      textContent: 'Close',
      style: 'float:right',
      onClick: () => { detailEl.className = 'hidden'; },
    }));
    detailEl.appendChild(U.el('div', { className: 'detail-panel-title', textContent: entry.title }));
    const meta = U.el('div', { className: 'detail-panel-meta' });
    meta.innerHTML = `${U.badge(entry.type)} ${U.scoreBar(entry.score)}`;
    detailEl.appendChild(meta);
    if (entry.url) {
      detailEl.appendChild(U.el('a', { className: 'url mb-8', href: entry.url, target: '_blank', textContent: entry.url, style: 'display:block' }));
    }
    const content = entry.content.length > 500 ? entry.content.slice(0, 500) + '...' : entry.content;
    detailEl.appendChild(U.el('div', { className: 'md-content', innerHTML: U.md(content) }));
  },
};
