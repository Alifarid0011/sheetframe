// Run after scripts/build-demo.sh: node --test docs/workflows.test.mjs
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test, before } from 'node:test';
import { workflows } from './workflows.mjs';

before(async () => {
  await import('./wasm_exec.js');
  const go = new globalThis.Go();
  const { instance } = await WebAssembly.instantiate(await readFile(new URL('./demo.wasm', import.meta.url)), go.importObject);
  // The Go demo intentionally stays alive to handle synchronous bridge calls.
  void go.run(instance);
});

function run(request) {
  const result = JSON.parse(globalThis.sheetframeRun(JSON.stringify(request)));
  assert.equal(result.error, undefined, result.error);
  return result;
}

async function apply(key) {
  const workflow = workflows[key];
  const csv = await readFile(new URL(workflow.source, import.meta.url), 'utf8');
  run({ action: 'load_csv', csv });
  return run(workflow.request);
}

test('sales workflow computes regional totals from the downloadable source in Go', async () => {
  const result = await apply('sales');
  assert.deepEqual(result.columns, ['region', 'sum_revenue']);
  assert.deepEqual(result.rows, [['East', 11520], ['West', 9160], ['North', 7950], ['South', 6420]]);
});

test('expense workflow groups and sorts the downloadable amounts in Go', async () => {
  const result = await apply('expenses');
  assert.deepEqual(result.columns, ['category', 'sum_amount']);
  assert.deepEqual(result.rows, [['Travel', 350], ['Software', 235], ['Office', 105]]);
});

test('cleanup workflow removes missing amounts while keeping zero and exports the result', async () => {
  const result = await apply('cleanup');
  assert.equal(result.totalRows, 4);
  assert.deepEqual(result.rows.map(row => [row[0], row[2]]), [['INV001', 1200], ['INV003', 0], ['INV004', 450], ['INV006', 800]]);
  const csv = Buffer.from(run({ action: 'export_csv' }).data, 'base64').toString();
  assert.match(csv, /INV003,Linen Co,0,credited/);
  assert.doesNotMatch(csv, /INV002|INV005/);
});
