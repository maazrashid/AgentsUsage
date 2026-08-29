const numberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });
const moneyFormat = new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 });

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

function renderRows(id, rows) {
  const target = document.getElementById(id);
  if (!target) return;
  if (!rows || rows.length === 0) {
    target.innerHTML = '<p class="empty">No usage found yet.</p>';
    return;
  }
  const maximum = Math.max(...rows.map((row) => row.totals.processedTokens), 1);
  target.replaceChildren(...rows.slice(0, 8).map((row) => {
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
    fill.className = 'fill';
    fill.style.width = `${Math.max(2, row.totals.processedTokens / maximum * 100)}%`;
    track.append(fill);
    wrapper.append(head, track);
    return wrapper;
  }));
}

function renderChart(rows) {
  const target = document.getElementById('daily-chart');
  if (!target) return;
  const byDay = new Map((rows || []).map((row) => [row.key, row.totals.processedTokens]));
  const days = [];
  const cursor = new Date();
  cursor.setHours(0, 0, 0, 0);
  cursor.setDate(cursor.getDate() - 29);
  for (let index = 0; index < 30; index += 1) {
    const key = `${cursor.getFullYear()}-${String(cursor.getMonth() + 1).padStart(2, '0')}-${String(cursor.getDate()).padStart(2, '0')}`;
    days.push({ key, value: byDay.get(key) || 0 });
    cursor.setDate(cursor.getDate() + 1);
  }
  const maximum = Math.max(...days.map((day) => day.value), 1);
  target.replaceChildren(...days.map((day) => {
    const wrapper = document.createElement('div');
    wrapper.className = 'bar-wrap';
    wrapper.dataset.tip = `${day.key} · ${compact(day.value)}`;
    const bar = document.createElement('div');
    bar.className = 'bar';
    bar.style.height = `${Math.max(1, day.value / maximum * 100)}%`;
    wrapper.append(bar);
    return wrapper;
  }));
}

function renderStats(stats) {
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
  setText('last-refresh', `Updated ${new Date(stats.generatedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`);
  setText('diagnostics', `${stats.diagnostics.filesScanned} files · ${stats.diagnostics.recordsParsed} usage records`);
  renderRows('providers', stats.providers);
  renderRows('models', stats.models);
  renderChart(stats.daily);
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

refresh();
setInterval(refresh, 10000);
