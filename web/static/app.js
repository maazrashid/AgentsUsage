const numberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });
const moneyFormat = new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 });
let latestStats = null;
const consumptionState = { provider: 'claude', period: 'today', metric: 'cost' };

function compact(value) {
  const absolute = Math.abs(value || 0);
  if (absolute >= 1e9) return `${numberFormat.format(value / 1e9)}B`;
  if (absolute >= 1e6) return `${numberFormat.format(value / 1e6)}M`;
  if (absolute >= 1e3) return `${numberFormat.format(value / 1e3)}K`;
  return numberFormat.format(value || 0);
}

function setText(id, value) {
  const element = document.getElementById(id);
  if (element) element.textContent = value;
}

function emptyMessage(text = 'No usage found yet.') {
  const element = document.createElement('p');
  element.className = 'empty';
  element.textContent = text;
  return element;
}

function renderRows(id, rows, limit = 8) {
  const target = document.getElementById(id);
  if (!target) return;
  if (!rows || rows.length === 0) {
    target.replaceChildren(emptyMessage());
    return;
  }
  const maximum = Math.max(...rows.map((row) => row.totals.processedTokens), 1);
  target.replaceChildren(...rows.slice(0, limit).map((row) => {
    const wrapper = document.createElement('div');
    wrapper.className = 'data-row';
    const head = document.createElement('div');
    head.className = 'row-head';
    const name = document.createElement('strong');
    name.textContent = row.key;
    const value = document.createElement('span');
    value.textContent = `${compact(row.totals.processedTokens)} · ${moneyFormat.format(row.totals.estimatedCostUSD)}`;
    head.append(name, value);
    const track = document.createElement('div');
    track.className = 'track';
    const fill = document.createElement('div');
    const provider = row.key.toLowerCase();
    fill.className = provider === 'claude' || provider === 'codex' ? `fill provider-${provider}` : 'fill';
    fill.style.width = `${Math.max(2, row.totals.processedTokens / maximum * 100)}%`;
    track.append(fill);
    wrapper.append(head, track);
    return wrapper;
  }));
}

function dateKey(date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function buildDailySeries(rows, count = 30, now = new Date()) {
  const byDay = new Map((rows || []).map((row) => [row.key, row.totals]));
  const cursor = new Date(now);
  cursor.setHours(0, 0, 0, 0);
  cursor.setDate(cursor.getDate() - count + 1);
  const days = [];
  for (let index = 0; index < count; index += 1) {
    const key = dateKey(cursor);
    days.push({ key, totals: byDay.get(key) || { processedTokens: 0, estimatedCostUSD: 0 } });
    cursor.setDate(cursor.getDate() + 1);
  }
  return days;
}

function renderBarChart(id, points, valueOf, tipOf, labelOf, labelEvery = 1) {
  const target = document.getElementById(id);
  if (!target) return;
  const maximum = Math.max(...points.map(valueOf), 1);
  target.replaceChildren(...points.map((point, index) => {
    const wrapper = document.createElement('div');
    wrapper.className = 'bar-wrap';
    const tip = tipOf(point);
    wrapper.dataset.tip = tip;
    wrapper.title = tip;
    wrapper.tabIndex = 0;
    wrapper.setAttribute('role', 'img');
    wrapper.setAttribute('aria-label', tip);
    const bar = document.createElement('div');
    bar.className = 'bar';
    const height = Math.max(1, valueOf(point) / maximum * 100);
    bar.style.height = `${height}%`;
    wrapper.style.setProperty('--bar-height', `${height}%`);
    const label = document.createElement('span');
    label.className = 'axis-label';
    label.textContent = index % labelEvery === 0 || index === points.length - 1 ? labelOf(point) : '';
    wrapper.append(bar, label);
    return wrapper;
  }));
}

function shortDateLabel(key) {
  const [, month, day] = String(key).split('-');
  return month && day ? `${Number(month)}/${Number(day)}` : key;
}

function renderCharts(stats) {
  const daily = buildDailySeries(stats.daily);
  const tokenValue = (point) => point.totals.processedTokens || 0;
  const tokenTip = (point) => `${point.key} · ${compact(tokenValue(point))} tokens`;
  renderBarChart('daily-chart', daily, tokenValue, tokenTip, (point) => shortDateLabel(point.key), 5);
  renderBarChart('trend-token-chart', daily, tokenValue, tokenTip, (point) => shortDateLabel(point.key), 5);
  renderBarChart('cost-chart', daily, (point) => point.totals.estimatedCostUSD || 0,
    (point) => `${point.key} · ${moneyFormat.format(point.totals.estimatedCostUSD || 0)}`,
    (point) => shortDateLabel(point.key), 5);

  const hourlyMap = new Map((stats.hourly || []).map((row) => [row.key, row.totals]));
  const hourly = Array.from({ length: 24 }, (_, hour) => {
    const key = `${String(hour).padStart(2, '0')}:00`;
    return { key, totals: hourlyMap.get(key) || { processedTokens: 0 } };
  });
  renderBarChart('hourly-chart', hourly, tokenValue,
    (point) => `${point.key} · ${compact(tokenValue(point))} tokens`,
    (point) => point.key.slice(0, 2), 4);
}

function resetLabel(value) {
  if (!value) return 'Starts with next use';
  const reset = new Date(value);
  if (Number.isNaN(reset.getTime())) return 'Reset time unavailable';
  const milliseconds = reset.getTime() - Date.now();
  if (milliseconds <= 0) return 'Reset due';
  const minutes = Math.ceil(milliseconds / 60000);
  const duration = minutes >= 2880 ? `${numberFormat.format(minutes / 1440)}d`
    : minutes >= 120 ? `${numberFormat.format(minutes / 60)}h`
      : `${minutes}m`;
  const clock = reset.toLocaleString([], milliseconds < 86400000
    ? { hour: '2-digit', minute: '2-digit' }
    : { weekday: 'short', hour: '2-digit', minute: '2-digit' });
  return `Resets in ${duration} · ${clock}`;
}

function windowTitle(window) {
  if (window.kind === 'session') return '5-hour window';
  if (window.kind === 'weekly') return 'Weekly window';
  return window.label || 'Scoped window';
}

function sourceLabel(snapshot) {
  if (snapshot.source === 'cli') return 'Live via CLI';
  if (snapshot.source === 'oauth') return 'Live via OAuth';
  return 'Last observed in local log';
}

function quotaCard(provider, snapshot, detailed) {
  const card = document.createElement('article');
  card.className = `quota-card quota-${provider}${detailed ? ' detailed' : ''}`;
  const header = document.createElement('div');
  header.className = 'quota-head';
  const title = document.createElement('div');
  const eyebrow = document.createElement('p');
  eyebrow.className = 'eyebrow';
  eyebrow.textContent = provider.toUpperCase();
  const heading = document.createElement('h3');
  heading.textContent = provider === 'claude' ? 'Claude Code' : 'Codex';
  title.append(eyebrow, heading);
  const badge = document.createElement('span');
  badge.className = `source-badge ${snapshot ? 'available' : ''}`;
  badge.textContent = snapshot ? sourceLabel(snapshot) : 'Unavailable';
  header.append(title, badge);
  card.append(header);
  if (!snapshot || !snapshot.windows || snapshot.windows.length === 0) {
    card.append(emptyMessage(provider === 'claude' ? 'No Claude OAuth quota is available.' : 'No Codex rate-limit snapshot is available.'));
    return card;
  }
  const windows = document.createElement('div');
  windows.className = 'quota-windows';
  snapshot.windows.forEach((window) => {
    const row = document.createElement('div');
    row.className = `quota-window${window.active ? ' binding' : ''}`;
    const line = document.createElement('div');
    line.className = 'quota-line';
    const label = document.createElement('strong');
    label.textContent = windowTitle(window);
    const percent = document.createElement('span');
    percent.textContent = `${numberFormat.format(window.usedPercent)}% used`;
    line.append(label, percent);
    const track = document.createElement('div');
    track.className = 'quota-track';
    const fill = document.createElement('div');
    const safePercent = Math.max(0, Math.min(100, window.usedPercent || 0));
    fill.className = safePercent >= 90 ? 'quota-fill danger' : safePercent >= 70 ? 'quota-fill warning' : 'quota-fill';
    fill.style.width = `${safePercent}%`;
    track.append(fill);
    row.append(line, track);
    if (detailed) {
      const meta = document.createElement('div');
      meta.className = 'quota-meta';
      meta.textContent = `${numberFormat.format(100 - safePercent)}% remaining · ${resetLabel(window.resetsAt)}`;
      row.append(meta);
    }
    windows.append(row);
  });
  card.append(windows);
  if (detailed) {
    const observed = document.createElement('p');
    observed.className = 'observed';
    observed.textContent = `Observed ${new Date(snapshot.observedAt).toLocaleString()}`;
    card.append(observed);
  }
  return card;
}

function renderQuotas(quotas) {
  const byProvider = new Map((quotas || []).map((value) => [value.provider, value]));
  const overview = document.getElementById('quota-overview');
  if (overview) overview.replaceChildren(quotaCard('claude', byProvider.get('claude'), false), quotaCard('codex', byProvider.get('codex'), false));
  const detail = document.getElementById('quota-detail');
  if (detail) detail.replaceChildren(quotaCard('claude', byProvider.get('claude'), true), quotaCard('codex', byProvider.get('codex'), true));
}

function renderComposition(totals) {
  const output = Math.max(0, totals.outputTokens || 0);
  const cacheRead = Math.max(0, totals.cacheReadTokens || 0);
  const cacheWrite = Math.max(0, totals.cacheWriteTokens || 0);
  const fresh = Math.max(0, (totals.processedTokens || 0) - output - cacheRead - cacheWrite);
  const parts = [
    { label: 'Fresh input', value: fresh, color: '#64d9ff' },
    { label: 'Cache read', value: cacheRead, color: '#73f5b1' },
    { label: 'Cache write', value: cacheWrite, color: '#ffb86b' },
    { label: 'Output', value: output, color: '#9f8cff' },
  ];
  const total = Math.max(parts.reduce((sum, part) => sum + part.value, 0), 1);
  let cursor = 0;
  const stops = parts.map((part) => {
    const start = cursor;
    cursor += part.value / total * 100;
    return `${part.color} ${start}% ${cursor}%`;
  });
  const donut = document.getElementById('token-donut');
  if (donut) donut.style.background = `conic-gradient(${stops.join(', ')})`;
  setText('donut-total', compact(totals.processedTokens));
  const legend = document.getElementById('token-legend');
  if (legend) legend.replaceChildren(...parts.map((part) => {
    const row = document.createElement('div');
    row.className = 'legend-row';
    const label = document.createElement('span');
    const dot = document.createElement('i');
    dot.style.background = part.color;
    label.textContent = part.label;
    const value = document.createElement('strong');
    value.textContent = compact(part.value);
    row.append(dot, label, value);
    return row;
  }));
}

function renderModelTable(models) {
  const body = document.getElementById('model-table');
  if (!body) return;
  if (!models || models.length === 0) {
    const row = document.createElement('tr');
    const cell = document.createElement('td');
    cell.colSpan = 6;
    cell.append(emptyMessage());
    row.append(cell);
    body.replaceChildren(row);
    return;
  }
  body.replaceChildren(...models.map((model) => {
    const row = document.createElement('tr');
    const values = [model.key, compact(model.totals.processedTokens), compact(model.totals.inputTokens),
      compact(model.totals.outputTokens), compact(model.totals.cacheReadTokens), moneyFormat.format(model.totals.estimatedCostUSD)];
    values.forEach((value, index) => {
      const cell = document.createElement(index === 0 ? 'th' : 'td');
      cell.textContent = value;
      if (index === 0) cell.scope = 'row';
      row.append(cell);
    });
    return row;
  }));
}

function tokenCompositionParts(totals = {}) {
  const output = Math.max(0, totals.outputTokens || 0);
  const cacheRead = Math.max(0, totals.cacheReadTokens || 0);
  const cacheWrite = Math.max(0, totals.cacheWriteTokens || 0);
  const fresh = Math.max(0, (totals.processedTokens || 0) - output - cacheRead - cacheWrite);
  return [
    { key: 'fresh', label: 'Fresh input', value: fresh, color: '#6ca9ff' },
    { key: 'cache', label: 'Cache read', value: cacheRead, color: '#45c7c9' },
    { key: 'write', label: 'Cache write', value: cacheWrite, color: '#ad8cff' },
    { key: 'output', label: 'Output', value: output, color: '#f0bd65' },
  ];
}

function cacheHitRate(totals = {}) {
  const parts = tokenCompositionParts(totals);
  const cacheRead = parts.find((part) => part.key === 'cache').value;
  const inputSide = parts.filter((part) => part.key !== 'output').reduce((sum, part) => sum + part.value, 0);
  return inputSide > 0 ? cacheRead / inputSide * 100 : 0;
}

function consumptionPeriodLabel(period) {
  return { today: 'Today', last30Days: 'Last 30 days', allTime: 'All time' }[period] || 'Today';
}

function createConsumptionKPI(label, value, detail, accent = false) {
  const card = document.createElement('article');
  card.className = `metric-card consumption-kpi${accent ? ' accent' : ''}`;
  const name = document.createElement('p');
  name.textContent = label;
  const amount = document.createElement('strong');
  amount.textContent = value;
  const caption = document.createElement('span');
  caption.textContent = detail;
  card.append(name, amount, caption);
  return card;
}

function renderConsumptionKPIs(period, provider) {
  const target = document.getElementById('consumption-kpis');
  if (!target) return;
  const totals = period.totals || {};
  const parts = tokenCompositionParts(totals);
  const fresh = parts.find((part) => part.key === 'fresh').value;
  const periodLabel = consumptionPeriodLabel(consumptionState.period).toLowerCase();
  const providerName = provider === 'claude' ? 'Claude Code' : 'Codex';
  const cards = [
    ['API-equivalent estimate', moneyFormat.format(totals.estimatedCostUSD || 0), `${numberFormat.format((totals.pricedTokens || 0) / Math.max(totals.processedTokens || 0, 1) * 100)}% pricing coverage`, true],
    ['Processed', compact(totals.processedTokens), periodLabel],
    ['Fresh input', compact(fresh), 'uncached usage'],
    ['Input', compact(totals.inputTokens), provider === 'codex' ? 'includes cached input' : 'uncached input tokens'],
    ['Cache read', compact(totals.cacheReadTokens), 'reused context'],
    ['Cache hit rate', `${numberFormat.format(cacheHitRate(totals))}%`, 'input-side tokens'],
    ['Output', compact(totals.outputTokens), 'generated tokens'],
    [provider === 'claude' ? 'Cache write' : 'Reasoning', compact(provider === 'claude' ? totals.cacheWriteTokens : totals.reasoningTokens), provider === 'claude' ? 'cache creation' : 'included in output'],
    ['Usage records', numberFormat.format(totals.usageRecords || 0), `${providerName} token observations`],
  ];
  target.replaceChildren(...cards.map((card) => createConsumptionKPI(...card)));
}

function createCompositionLegend(parts, totals) {
  const total = Math.max(parts.reduce((sum, part) => sum + part.value, 0), 1);
  return parts.filter((part) => part.value > 0).map((part) => {
    const row = document.createElement('span');
    const dot = document.createElement('i');
    dot.style.background = part.color;
    const label = document.createElement('span');
    label.textContent = `${part.label} ${compact(part.value)} (${numberFormat.format(part.value / total * 100)}%)`;
    row.append(dot, label);
    return row;
  });
}

function renderConsumptionComposition(totals) {
  const bar = document.getElementById('consumption-composition-bar');
  const legend = document.getElementById('consumption-composition-legend');
  if (!bar || !legend) return;
  const parts = tokenCompositionParts(totals);
  const total = Math.max(parts.reduce((sum, part) => sum + part.value, 0), 1);
  bar.replaceChildren(...parts.filter((part) => part.value > 0).map((part) => {
    const segment = document.createElement('span');
    segment.className = `composition-segment ${part.key}`;
    segment.style.width = `${part.value / total * 100}%`;
    segment.style.background = part.color;
    segment.title = `${part.label}: ${numberFormat.format(part.value)} tokens`;
    return segment;
  }));
  legend.replaceChildren(...createCompositionLegend(parts, totals));
  setText('consumption-composition-period', consumptionPeriodLabel(consumptionState.period).toUpperCase());
  setText('consumption-reasoning-note', `${compact(totals.reasoningTokens || 0)} reasoning tokens · included in output`);
}

function modelDetailItem(label, value) {
  const item = document.createElement('div');
  const name = document.createElement('span');
  name.textContent = label;
  const amount = document.createElement('strong');
  amount.textContent = value;
  item.append(name, amount);
  return item;
}

function renderConsumptionModels(models) {
  const target = document.getElementById('consumption-models');
  if (!target) return;
  if (!models || models.length === 0) {
    target.replaceChildren(emptyMessage('No model usage was found for this period.'));
    return;
  }
  target.replaceChildren(...models.map((model, index) => {
    const details = document.createElement('details');
    details.className = 'consumption-model';
    if (index === 0) details.open = true;
    const summary = document.createElement('summary');
    const name = document.createElement('strong');
    name.textContent = model.key;
    const total = document.createElement('span');
    total.textContent = `${compact(model.totals.processedTokens)} · ${moneyFormat.format(model.totals.estimatedCostUSD)}`;
    summary.append(name, total);
    const body = document.createElement('div');
    body.className = 'model-detail-grid';
    const parts = tokenCompositionParts(model.totals);
    body.append(
      modelDetailItem('Processed', numberFormat.format(model.totals.processedTokens || 0)),
      modelDetailItem('Fresh input', numberFormat.format(parts[0].value)),
      modelDetailItem('Input', numberFormat.format(model.totals.inputTokens || 0)),
      modelDetailItem('Cache read', numberFormat.format(model.totals.cacheReadTokens || 0)),
      modelDetailItem('Cache write', numberFormat.format(model.totals.cacheWriteTokens || 0)),
      modelDetailItem('Output', numberFormat.format(model.totals.outputTokens || 0)),
      modelDetailItem('Reasoning', numberFormat.format(model.totals.reasoningTokens || 0)),
      modelDetailItem('Cache hit rate', `${numberFormat.format(cacheHitRate(model.totals))}%`),
      modelDetailItem('Usage records', numberFormat.format(model.totals.usageRecords || 0)),
      modelDetailItem('Estimate', moneyFormat.format(model.totals.estimatedCostUSD || 0)),
    );
    details.append(summary, body);
    return details;
  }));
}

function emptyTimelineTotals() {
  return { inputTokens: 0, outputTokens: 0, cacheReadTokens: 0, cacheWriteTokens: 0, reasoningTokens: 0, processedTokens: 0, usageRecords: 0, estimatedCostUSD: 0, pricedTokens: 0 };
}

function consumptionTimeline(rows, period) {
  if (period === 'today') {
    const byHour = new Map((rows || []).map((row) => [row.key, row.totals]));
    return Array.from({ length: 24 }, (_, hour) => {
      const key = `${String(hour).padStart(2, '0')}:00`;
      return { key, totals: byHour.get(key) || emptyTimelineTotals() };
    });
  }
  if (period === 'last30Days') return buildDailySeries(rows || []);
  return rows || [];
}

const consumptionMetrics = {
  cost: { label: 'API-equivalent estimate', value: (totals) => totals.estimatedCostUSD || 0, format: (value) => moneyFormat.format(value), className: 'cost' },
  processed: { label: 'Processed tokens', value: (totals) => totals.processedTokens || 0, format: (value) => `${compact(value)} tokens`, className: 'token' },
  fresh: { label: 'Fresh input', value: (totals) => tokenCompositionParts(totals)[0].value, format: (value) => `${compact(value)} tokens`, className: 'fresh' },
  cache: { label: 'Cache read', value: (totals) => totals.cacheReadTokens || 0, format: (value) => `${compact(value)} tokens`, className: 'cache' },
  output: { label: 'Output tokens', value: (totals) => totals.outputTokens || 0, format: (value) => `${compact(value)} tokens`, className: 'output' },
  reasoning: { label: 'Reasoning tokens', value: (totals) => totals.reasoningTokens || 0, format: (value) => `${compact(value)} tokens`, className: 'reasoning' },
  records: { label: 'Usage records', value: (totals) => totals.usageRecords || 0, format: (value) => `${numberFormat.format(value)} records`, className: 'records' },
};

function timelineAxisLabel(point, period) {
  return period === 'today' ? point.key.slice(0, 2) : shortDateLabel(point.key);
}

function timelineLabelEvery(length, period) {
  if (period === 'today') return 4;
  return Math.max(1, Math.ceil(length / 7));
}

function renderConsumptionMetricChart(timeline, provider) {
  const metric = consumptionMetrics[consumptionState.metric] || consumptionMetrics.cost;
  const chartRows = consumptionState.period === 'allTime' ? timeline.slice(-60) : timeline;
  const target = document.getElementById('consumption-chart');
  if (target) target.className = `chart consumption-chart metric-${metric.className} provider-${provider}`;
  renderBarChart('consumption-chart', chartRows, (row) => metric.value(row.totals),
    (row) => `${row.key} · ${metric.format(metric.value(row.totals))}`,
    (row) => timelineAxisLabel(row, consumptionState.period), timelineLabelEvery(chartRows.length, consumptionState.period));
  setText('consumption-timeline-title', metric.label);
  setText('consumption-timeline-period', consumptionState.period === 'today' ? 'TODAY BY HOUR' : `${consumptionPeriodLabel(consumptionState.period).toUpperCase()} · DAILY`);
  const legend = document.getElementById('consumption-chart-legend');
  if (legend) {
    const label = document.createElement('span');
    const dot = document.createElement('i');
    dot.className = `legend-swatch ${metric.className}`;
    label.append(dot, document.createTextNode(metric.label));
    const hint = document.createElement('span');
    hint.className = 'chart-hint';
    hint.textContent = consumptionState.period === 'allTime' && timeline.length > chartRows.length ? `Latest ${chartRows.length} active days` : consumptionPeriodLabel(consumptionState.period);
    legend.replaceChildren(label, hint);
  }
}

function renderConsumptionStackChart(timeline) {
  const target = document.getElementById('consumption-stack-chart');
  const legend = document.getElementById('consumption-stack-legend');
  if (!target || !legend) return;
  const rows = consumptionState.period === 'allTime' ? timeline.slice(-60) : timeline;
  const maximum = Math.max(...rows.map((row) => row.totals.processedTokens || 0), 1);
  target.replaceChildren(...rows.map((row, index) => {
    const parts = tokenCompositionParts(row.totals);
    const total = Math.max(parts.reduce((sum, part) => sum + part.value, 0), 1);
    const height = Math.max(1, (row.totals.processedTokens || 0) / maximum * 100);
    const wrapper = document.createElement('div');
    wrapper.className = 'bar-wrap stack-wrap';
    const tip = `${row.key} · ${parts.filter((part) => part.value > 0).map((part) => `${part.label} ${compact(part.value)}`).join(' · ') || 'No usage'}`;
    wrapper.dataset.tip = tip;
    wrapper.title = tip;
    wrapper.tabIndex = 0;
    wrapper.setAttribute('role', 'img');
    wrapper.setAttribute('aria-label', tip);
    wrapper.style.setProperty('--bar-height', `${height}%`);
    const stack = document.createElement('div');
    stack.className = 'stack-bar';
    stack.style.height = `${height}%`;
    parts.filter((part) => part.value > 0).forEach((part) => {
      const segment = document.createElement('span');
      segment.style.height = `${part.value / total * 100}%`;
      segment.style.background = part.color;
      stack.append(segment);
    });
    const label = document.createElement('span');
    label.className = 'axis-label';
    label.textContent = index % timelineLabelEvery(rows.length, consumptionState.period) === 0 || index === rows.length - 1
      ? timelineAxisLabel(row, consumptionState.period) : '';
    wrapper.append(stack, label);
    return wrapper;
  }));
  const totals = rows.reduce((sum, row) => {
    tokenCompositionParts(row.totals).forEach((part, index) => { sum[index].value += part.value; });
    return sum;
  }, tokenCompositionParts({}));
  legend.replaceChildren(...createCompositionLegend(totals));
}

function renderConsumptionTable(timeline) {
  const body = document.getElementById('consumption-table');
  if (!body) return;
  const rows = [...timeline].reverse();
  if (rows.length === 0) {
    const row = document.createElement('tr');
    const cell = document.createElement('td');
    cell.colSpan = 10;
    cell.append(emptyMessage('No activity was found for this period.'));
    row.append(cell);
    body.replaceChildren(row);
    return;
  }
  body.replaceChildren(...rows.map((entry) => {
    const row = document.createElement('tr');
    const totals = entry.totals;
    const fresh = tokenCompositionParts(totals)[0].value;
    const values = [entry.key, moneyFormat.format(totals.estimatedCostUSD || 0), numberFormat.format(totals.processedTokens || 0),
      numberFormat.format(fresh), numberFormat.format(totals.inputTokens || 0), numberFormat.format(totals.cacheReadTokens || 0),
      `${numberFormat.format(cacheHitRate(totals))}%`, numberFormat.format(totals.outputTokens || 0),
      numberFormat.format(totals.reasoningTokens || 0), numberFormat.format(totals.usageRecords || 0)];
    values.forEach((value, index) => {
      const cell = document.createElement(index === 0 ? 'th' : 'td');
      cell.textContent = value;
      if (index === 0) cell.scope = 'row';
      row.append(cell);
    });
    return row;
  }));
  const hourly = consumptionState.period === 'today';
  setText('consumption-table-title', hourly ? 'Hourly details' : 'Daily details');
  setText('consumption-time-heading', hourly ? 'Hour' : 'Date');
}

function updateConsumptionControls() {
  document.querySelectorAll('[data-consumption-provider]').forEach((button) => {
    const selected = button.dataset.consumptionProvider === consumptionState.provider;
    button.classList.toggle('active', selected);
    button.setAttribute('aria-pressed', String(selected));
  });
  document.querySelectorAll('[data-consumption-period]').forEach((button) => {
    const selected = button.dataset.consumptionPeriod === consumptionState.period;
    button.classList.toggle('active', selected);
    button.setAttribute('aria-pressed', String(selected));
  });
  document.querySelectorAll('[data-consumption-metric]').forEach((button) => {
    const selected = button.dataset.consumptionMetric === consumptionState.metric;
    button.classList.toggle('active', selected);
    button.setAttribute('aria-pressed', String(selected));
  });
}

function renderConsumption(stats) {
  const provider = (stats.consumption || []).find((entry) => entry.provider === consumptionState.provider);
  if (!provider) return;
  const period = provider[consumptionState.period] || provider.today;
  const timeline = consumptionTimeline(period.timeline, consumptionState.period);
  updateConsumptionControls();
  renderConsumptionKPIs(period, provider.provider);
  renderConsumptionComposition(period.totals || {});
  renderConsumptionModels(period.models || []);
  renderConsumptionMetricChart(timeline, provider.provider);
  renderConsumptionStackChart(timeline);
  renderConsumptionTable(timeline);
}

function setupConsumptionControls() {
  const savedProvider = localStorage.getItem('agentsusage.consumptionProvider');
  const savedPeriod = localStorage.getItem('agentsusage.consumptionPeriod');
  const savedMetric = localStorage.getItem('agentsusage.consumptionMetric');
  if (['claude', 'codex'].includes(savedProvider)) consumptionState.provider = savedProvider;
  if (['today', 'last30Days', 'allTime'].includes(savedPeriod)) consumptionState.period = savedPeriod;
  if (Object.hasOwn(consumptionMetrics, savedMetric)) consumptionState.metric = savedMetric;
  document.querySelectorAll('[data-consumption-provider]').forEach((button) => button.addEventListener('click', () => {
    consumptionState.provider = button.dataset.consumptionProvider;
    localStorage.setItem('agentsusage.consumptionProvider', consumptionState.provider);
    if (latestStats) renderConsumption(latestStats);
  }));
  document.querySelectorAll('[data-consumption-period]').forEach((button) => button.addEventListener('click', () => {
    consumptionState.period = button.dataset.consumptionPeriod;
    localStorage.setItem('agentsusage.consumptionPeriod', consumptionState.period);
    if (latestStats) renderConsumption(latestStats);
  }));
  document.querySelectorAll('[data-consumption-metric]').forEach((button) => button.addEventListener('click', () => {
    consumptionState.metric = button.dataset.consumptionMetric;
    localStorage.setItem('agentsusage.consumptionMetric', consumptionState.metric);
    if (latestStats) renderConsumption(latestStats);
  }));
  updateConsumptionControls();
}

function renderStats(stats) {
  latestStats = stats;
  const today = stats.today;
  const inputSide = today.inputTokens + today.cacheReadTokens + today.cacheWriteTokens;
  const coverage = today.processedTokens > 0 ? today.pricedTokens / today.processedTokens * 100 : 0;
  const cacheRate = inputSide > 0 ? today.cacheReadTokens / inputSide * 100 : 0;
  setText('today-tokens', compact(today.processedTokens));
  setText('today-cost', moneyFormat.format(today.estimatedCostUSD));
  setText('price-coverage', `Pricing coverage ${numberFormat.format(coverage)}%`);
  setText('input-tokens', compact(today.inputTokens));
  setText('output-tokens', compact(today.outputTokens));
  setText('cache-tokens', compact(today.cacheReadTokens));
  setText('cache-rate', `${numberFormat.format(cacheRate)}% of input-side tokens`);
  setText('week-tokens', compact(stats.last7Days.processedTokens));
  setText('week-cost', `${moneyFormat.format(stats.last7Days.estimatedCostUSD)} estimated`);
  setText('trend-week-tokens', compact(stats.last7Days.processedTokens));
  setText('trend-week-average', `${compact(stats.last7Days.processedTokens / 7)} daily average`);
  setText('trend-week-cost', moneyFormat.format(stats.last7Days.estimatedCostUSD));
  setText('all-time-tokens', compact(stats.allTime.processedTokens));
  setText('reasoning-tokens', compact(stats.allTime.reasoningTokens));
  setText('last-refresh', `Updated ${new Date(stats.generatedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`);
  setText('diagnostics', `${stats.diagnostics.filesScanned} files · ${stats.diagnostics.recordsParsed} usage records · ${compact(stats.diagnostics.bytesReadThisRefresh)}B read`);
  renderRows('providers', stats.providers);
  renderRows('provider-detail', stats.providers, 20);
  renderRows('models', stats.models);
  renderCharts(stats);
  renderQuotas(stats.quotas);
  renderComposition(stats.allTime);
  renderModelTable(stats.models);
  renderConsumption(stats);
}

function activateTab(name, persist = true) {
  const tabs = Array.from(document.querySelectorAll('[role="tab"][data-tab]'));
  const target = tabs.find((tab) => tab.dataset.tab === name) || tabs[0];
  if (!target) return;
  tabs.forEach((tab) => {
    const selected = tab === target;
    tab.classList.toggle('active', selected);
    tab.setAttribute('aria-selected', String(selected));
    tab.tabIndex = selected ? 0 : -1;
    const panel = document.getElementById(tab.dataset.tab);
    if (panel) {
      panel.hidden = !selected;
      panel.classList.toggle('active', selected);
    }
  });
  revealTab(target, persist);
  if (persist) localStorage.setItem('agentsusage.activeTab', target.dataset.tab);
}

function revealTab(target, smooth) {
  const strip = target.parentElement;
  if (!strip || strip.scrollWidth <= strip.clientWidth) return;
  const left = target.offsetLeft - (strip.clientWidth - target.offsetWidth) / 2;
  strip.scrollTo({ left: Math.max(0, left), behavior: smooth ? 'smooth' : 'auto' });
}

function setupTabs() {
  const tabs = Array.from(document.querySelectorAll('[role="tab"][data-tab]'));
  tabs.forEach((tab, index) => {
    tab.addEventListener('click', () => activateTab(tab.dataset.tab));
    tab.addEventListener('keydown', (event) => {
      if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
      event.preventDefault();
      let next = index;
      if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
      if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
      if (event.key === 'Home') next = 0;
      if (event.key === 'End') next = tabs.length - 1;
      activateTab(tabs[next].dataset.tab);
      tabs[next].focus();
    });
  });
  activateTab(localStorage.getItem('agentsusage.activeTab') || 'overview', false);
}

async function refresh() {
  const dot = document.getElementById('status-dot');
  try {
    const [statusResponse, statsResponse] = await Promise.all([fetch('/api/status'), fetch('/api/stats')]);
    if (!statusResponse.ok || !statsResponse.ok) throw new Error('server returned an error');
    const status = await statusResponse.json();
    const stats = await statsResponse.json();
    renderStats(stats);
    dot.className = status.lastError ? 'status-dot error' : 'status-dot live';
    setText('status-label', status.lastError ? 'Running with scan warning' : 'Live');
  } catch (error) {
    dot.className = 'status-dot error';
    setText('status-label', 'Disconnected');
  }
}

if (typeof document !== 'undefined') {
  setupTabs();
  setupConsumptionControls();
  refresh();
  setInterval(refresh, 10000);
}

if (typeof module !== 'undefined') {
  module.exports = { buildDailySeries, cacheHitRate, consumptionTimeline, resetLabel, shortDateLabel, sourceLabel, tokenCompositionParts, windowTitle };
}
