const History = {
  async render(container) {
    const controls = U.el('div', { className: 'card flex gap-8', style: 'align-items:center' });
    controls.appendChild(U.el('span', { className: 'text-slate', textContent: 'Filter by topic:' }));
    const topicSel = U.topicSelector('');
    topicSel.id = 'history-topic';
    topicSel.addEventListener('change', () => this.loadSessions());
    controls.appendChild(topicSel);
    container.appendChild(controls);

    // Sparkline container
    const sparkEl = U.el('div', {
      id: 'history-spark',
      className: 'card',
      style: 'height:120px;padding:12px',
    });
    container.appendChild(sparkEl);

    // Sessions table
    const tableEl = U.el('div', { id: 'history-table' });
    container.appendChild(tableEl);

    await this.loadSessions();
  },

  async loadSessions() {
    const topicSlug = document.getElementById('history-topic')?.value || '';
    const url = topicSlug ? `/api/sessions?topic=${topicSlug}` : '/api/sessions';
    const sessions = await API.get(url);

    this.renderSparkline(sessions || []);
    this.renderTable(sessions || []);
  },

  renderSparkline(sessions) {
    const container = document.getElementById('history-spark');
    if (!container || typeof d3 === 'undefined') return;
    container.innerHTML = '';

    if (sessions.length < 2) {
      container.innerHTML = '<div class="empty-state text-sm">Not enough data for timeline.</div>';
      return;
    }

    const width = container.clientWidth - 24;
    const height = 96;

    // Group by day
    const dayMap = new Map();
    for (const s of sessions) {
      const day = s.timestamp.split('T')[0];
      dayMap.set(day, (dayMap.get(day) || 0) + 1);
    }
    const data = Array.from(dayMap.entries())
      .map(([day, count]) => ({ day: new Date(day), count }))
      .sort((a, b) => a.day - b.day);

    const svg = d3.select(container).append('svg')
      .attr('width', width)
      .attr('height', height);

    const x = d3.scaleTime()
      .domain(d3.extent(data, d => d.day))
      .range([30, width - 10]);

    const y = d3.scaleLinear()
      .domain([0, d3.max(data, d => d.count)])
      .range([height - 20, 10]);

    // Area
    svg.append('path')
      .datum(data)
      .attr('fill', 'rgba(201,168,76,0.15)')
      .attr('d', d3.area()
        .x(d => x(d.day))
        .y0(height - 20)
        .y1(d => y(d.count))
        .curve(d3.curveMonotoneX)
      );

    // Line
    svg.append('path')
      .datum(data)
      .attr('fill', 'none')
      .attr('stroke', '#C9A84C')
      .attr('stroke-width', 2)
      .attr('d', d3.line()
        .x(d => x(d.day))
        .y(d => y(d.count))
        .curve(d3.curveMonotoneX)
      );

    // Dots
    svg.selectAll('circle').data(data).join('circle')
      .attr('cx', d => x(d.day))
      .attr('cy', d => y(d.count))
      .attr('r', 3)
      .attr('fill', '#C9A84C');

    // X axis
    svg.append('g')
      .attr('transform', `translate(0,${height - 20})`)
      .call(d3.axisBottom(x).ticks(5).tickFormat(d3.timeFormat('%b %d')))
      .selectAll('text').attr('fill', '#8899AA').attr('font-size', 10);

    svg.selectAll('.domain, .tick line').attr('stroke', '#253553');
  },

  renderTable(sessions) {
    const container = document.getElementById('history-table');
    if (!container) return;
    container.innerHTML = '';

    if (!sessions.length) {
      container.innerHTML = '<div class="empty-state">No research sessions recorded.</div>';
      return;
    }

    const card = U.el('div', { className: 'card' });
    card.appendChild(U.el('div', { className: 'card-header', textContent: `${sessions.length} sessions` }));

    const table = U.el('table');
    table.innerHTML = '<thead><tr><th>Query</th><th>Resolution</th><th>Results</th><th>When</th><th></th></tr></thead>';
    const tbody = U.el('tbody');

    for (const s of sessions) {
      const tr = U.el('tr');
      tr.innerHTML = `
        <td class="text-cream">${s.query}</td>
        <td><span class="badge badge-source">${s.resolution}</span></td>
        <td>${s.result_count}</td>
        <td class="text-slate text-sm">${U.timeAgo(s.timestamp)}</td>
      `;
      const actionTd = U.el('td');
      actionTd.appendChild(U.el('button', {
        className: 'btn btn-sm btn-ghost',
        textContent: 'Re-run',
        onClick: () => { App.navigate('research'); Research.prefill(s.query, s.resolution); },
      }));
      tr.appendChild(actionTd);
      tbody.appendChild(tr);
    }

    table.appendChild(tbody);
    card.appendChild(table);
    container.appendChild(card);
  },
};
