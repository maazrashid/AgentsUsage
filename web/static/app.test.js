const test = require('node:test');
const assert = require('node:assert/strict');

const {
  buildDailySeries, cacheHitRate, consumptionTimeline, resetLabel, shortDateLabel,
  sourceLabel, tokenCompositionParts, windowTitle,
} = require('./app.js');

test('buildDailySeries fills missing calendar days', () => {
  const rows = [{ key: '2026-08-28', totals: { processedTokens: 42, estimatedCostUSD: 0.12 } }];
  const result = buildDailySeries(rows, 3, new Date(2026, 7, 29, 12));

  assert.deepEqual(result.map((row) => row.key), ['2026-08-27', '2026-08-28', '2026-08-29']);
  assert.equal(result[0].totals.processedTokens, 0);
  assert.equal(result[1].totals.processedTokens, 42);
});

test('quota labels distinguish account-wide windows', () => {
  assert.equal(windowTitle({ kind: 'session', label: 'primary' }), '5-hour window');
  assert.equal(windowTitle({ kind: 'weekly', label: 'secondary' }), 'Weekly window');
  assert.equal(windowTitle({ kind: 'weekly_scoped', label: 'Opus' }), 'Opus');
});

test('quota source labels explain freshness', () => {
  assert.equal(sourceLabel({ source: 'cli' }), 'Live via CLI');
  assert.equal(sourceLabel({ source: 'oauth' }), 'Live via OAuth');
  assert.equal(sourceLabel({ source: 'local-log' }), 'Last observed in local log');
});

test('resetLabel handles an absent timestamp without claiming a reset', () => {
  assert.equal(resetLabel(null), 'Starts with next use');
  assert.equal(resetLabel('not-a-date'), 'Reset time unavailable');
});

test('shortDateLabel produces compact chart-axis labels', () => {
  assert.equal(shortDateLabel('2026-08-29'), '8/29');
  assert.equal(shortDateLabel('unknown'), 'unknown');
});

test('tokenCompositionParts avoids counting cache twice', () => {
  const parts = tokenCompositionParts({ processedTokens: 150, outputTokens: 20, cacheReadTokens: 80, cacheWriteTokens: 10 });
  assert.deepEqual(parts.map((part) => part.value), [40, 80, 10, 20]);
  assert.equal(cacheHitRate({ processedTokens: 150, outputTokens: 20, cacheReadTokens: 80, cacheWriteTokens: 10 }), 80 / 130 * 100);
});

test('consumptionTimeline fills all hours for today', () => {
  const timeline = consumptionTimeline([{ key: '09:00', totals: { processedTokens: 12 } }], 'today');
  assert.equal(timeline.length, 24);
  assert.equal(timeline[9].totals.processedTokens, 12);
  assert.equal(timeline[10].totals.processedTokens, 0);
});
