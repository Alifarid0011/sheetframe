/* Sheetframe's UI delegates every data operation to the real Go/WASM bridge. */
'use strict';
(() => {
  const $ = id => document.getElementById(id);
  let ready = false;
  let busy = false;
  let schema = [];
  let sourceName = 'studio_orders.csv';
  const actionIDs = ['sample-button', 'load-csv', 'apply-button', 'clear-button', 'insight-button', 'download-csv', 'download-xlsx', 'file-input'];
  const operationIDs = ['filter-enabled', 'sort-enabled', 'group-enabled'];
  const selectors = ['filter-column', 'sort-column', 'group-column', 'agg-column'];

  function error(message = '') {
    $('error-message').textContent = message;
    $('error-message').hidden = !message;
  }

  function syncControls() {
    const disabled = !ready || busy;
    syncSortColumns();
    actionIDs.forEach(id => { $(id).disabled = disabled; });
    operationIDs.forEach(id => { $(id).disabled = disabled; });
    ['filter-column', 'filter-op', 'filter-value'].forEach(id => { $(id).disabled = disabled || !$('filter-enabled').checked; });
    ['sort-column', 'sort-direction'].forEach(id => { $(id).disabled = disabled || !$('sort-enabled').checked; });
    ['group-column', 'agg-op', 'agg-column'].forEach(id => { $(id).disabled = disabled || !$('group-enabled').checked; });
    const nullOperation = ['is_null', 'not_null'].includes($('filter-op').value);
    $('filter-value').disabled ||= nullOperation;
    $('filter-value-label').hidden = nullOperation;
    $('workspace').setAttribute('aria-busy', String(busy || !ready));
    $('apply-button').firstChild.textContent = busy ? 'Working… ' : 'Apply transformation ';
  }

  function syncSortColumns() {
    const names = $('group-enabled').checked
      ? [$('group-column').value, `${$('agg-op').value}_${$('agg-column').value}`]
      : schema.map(field => field.name);
    const select = $('sort-column');
    if (Array.from(select.options).map(option => option.value).join('\0') === names.join('\0')) return;
    const previous = select.value;
    select.replaceChildren(...names.map(name => {
      const option = document.createElement('option');
      option.value = name;
      option.textContent = name;
      return option;
    }));
    if (names.includes(previous)) select.value = previous;
    else if ($('group-enabled').checked) select.value = names[1];
  }

  function bridge(request) {
    const response = JSON.parse(window.sheetframeRun(JSON.stringify(request)));
    if (response.error) throw new Error(response.error);
    return response;
  }

  async function execute(request, { loadName, badge = 'Original dataset' } = {}) {
    if (!ready || busy) return;
    busy = true;
    error();
    syncControls();
    // Let the browser paint the pending state before entering synchronous Go.
    await new Promise(resolve => requestAnimationFrame(() => setTimeout(resolve, 0)));
    try {
      const response = bridge(request);
      if (loadName) {
        sourceName = loadName;
        schema = response.schema || [];
        $('dataset-name').textContent = sourceName;
        $('source-tag').textContent = sourceName === 'studio_orders.csv' ? 'SAMPLE' : 'LOCAL';
        $('source-shape').textContent = `${formatCount(response.totalRows)} rows · ${response.columns.length} columns`;
        $('result-title').textContent = sourceName === 'studio_orders.csv' ? 'Studio orders' : sourceName.replace(/\.(csv|xlsx)$/i, '');
        populateColumns();
        operationIDs.forEach(id => { $(id).checked = false; });
      }
      render(response);
      $('result-badge').textContent = badge;
      return response;
    } catch (cause) {
      error(cause.message || 'The operation could not be completed. Please check your data and try again.');
    } finally {
      busy = false;
      syncControls();
    }
  }

  function formatCount(value) { return Number(value || 0).toLocaleString(); }

  function populateColumns() {
    selectors.forEach(id => {
      const select = $(id);
      select.replaceChildren(...schema.map(field => {
        const option = document.createElement('option');
        option.value = field.name;
        option.textContent = field.name;
        return option;
      }));
    });
    const numeric = schema.find(field => String(field.kind).toLowerCase() === 'number');
    if (numeric) $('agg-column').value = numeric.name;
    $('group-column').value = schema.find(field => String(field.kind).toLowerCase() === 'string')?.name || schema[0]?.name || '';
    if (schema.some(field => field.name === 'region')) $('group-column').value = 'region';
    if (schema.some(field => field.name === 'revenue')) $('agg-column').value = 'revenue';
  }

  function render(response) {
    $('row-count').textContent = formatCount(response.totalRows);
    $('column-count').textContent = formatCount(response.columns.length);
    const elapsed = Number(response.elapsedMS);
    $('elapsed').textContent = Number.isFinite(elapsed) ? `${elapsed.toLocaleString(undefined, { maximumFractionDigits: 2 })} ms` : '—';
    const header = document.createElement('tr');
    const rowNumber = document.createElement('th');
    rowNumber.textContent = '#';
    rowNumber.scope = 'col';
    rowNumber.className = 'row-number';
    header.append(rowNumber);
    response.columns.forEach((name, index) => {
      const th = document.createElement('th');
      th.scope = 'col';
      th.textContent = name;
      const type = document.createElement('span');
      type.className = 'column-type';
      type.textContent = response.schema?.[index]?.kind || 'unknown';
      th.append(type);
      header.append(th);
    });
    const body = document.createDocumentFragment();
    (response.rows || []).slice(0, 200).forEach((record, index) => {
      const tr = document.createElement('tr');
      const n = document.createElement('td');
      n.className = 'row-number';
      n.textContent = String(index + 1).padStart(2, '0');
      tr.append(n);
      record.forEach(value => {
        const td = document.createElement('td');
        td.textContent = value === null ? 'null' : String(value);
        if (value === null) td.className = 'null-value';
        else if (typeof value === 'number') td.className = 'number-cell';
        td.title = td.textContent;
        tr.append(td);
      });
      body.append(tr);
    });
    if (!response.rows?.length) {
      const tr = document.createElement('tr');
      const td = document.createElement('td');
      td.colSpan = response.columns.length + 1;
      td.className = 'loading-cell';
      td.textContent = 'No rows match this view. Adjust a filter or clear transformations.';
      tr.append(td);
      body.append(tr);
    }
    $('data-table').tHead.replaceChildren(header);
    $('data-table').tBodies[0].replaceChildren(body);
    const visible = Math.min(response.rows?.length || 0, 200);
    $('preview-note').textContent = `${visible ? `Showing 1–${formatCount(visible)}` : 'Showing 0'} of ${formatCount(response.totalRows)} rows${response.totalRows > 200 ? ' · exports include every row' : ''}`;
  }

  function buildTransform() {
    const request = { action: 'transform' };
    if ($('filter-enabled').checked) {
      const column = $('filter-column').value;
      const op = $('filter-op').value;
      const raw = $('filter-value').value;
      let value = raw;
      const kind = String(schema.find(field => field.name === column)?.kind).toLowerCase();
      if (!['is_null', 'not_null'].includes(op)) {
        if (kind === 'number') {
          if (!raw.trim() || !Number.isFinite(Number(raw))) throw new Error('Enter a valid number for this numeric column.');
          value = Number(raw);
        } else if (kind === 'bool') {
          if (!['true', 'false'].includes(raw.toLowerCase())) throw new Error('Enter true or false for this boolean column.');
          value = raw.toLowerCase() === 'true';
        }
      }
      request.filter = { column, op, value };
    }
    if ($('sort-enabled').checked) request.sort = { column: $('sort-column').value, descending: $('sort-direction').value === 'desc' };
    if ($('group-enabled').checked) {
      const column = $('agg-column').value;
      const op = $('agg-op').value;
      request.group = { keys: [$('group-column').value], aggregations: [{ column, op, as: `${op}_${column}` }] };
    }
    return request;
  }

  async function loadSample() {
    try {
      const response = await fetch('sample.csv');
      if (!response.ok) throw new Error('The sample CSV could not be loaded. You can still import your own file.');
      const csv = await response.text();
      $('csv-input').value = csv;
      return await execute({ action: 'load_csv', csv }, { loadName: 'studio_orders.csv' });
    } catch (cause) { error(cause.message); }
  }

  $('transform-form').addEventListener('submit', event => {
    event.preventDefault();
    try {
      const request = buildTransform();
      const operations = ['filter', 'sort', 'group'].filter(key => request[key]);
      execute(request, { badge: operations.length ? operations.map(key => ({ filter: 'Filtered', sort: 'Sorted', group: 'Grouped' })[key]).join(' · ') : 'Original dataset' });
    } catch (cause) { error(cause.message); }
  });
  operationIDs.forEach(id => $(id).addEventListener('change', syncControls));
  $('filter-op').addEventListener('change', syncControls);
  ['group-column', 'agg-column', 'agg-op'].forEach(id => $(id).addEventListener('change', syncControls));
  $('sample-button').addEventListener('click', loadSample);
  $('load-csv').addEventListener('click', () => execute({ action: 'load_csv', csv: $('csv-input').value }, { loadName: 'pasted_data.csv' }));
  $('clear-button').addEventListener('click', async () => {
    const response = await execute({ action: 'transform' });
    if (response) {
      operationIDs.forEach(id => { $(id).checked = false; });
      syncControls();
    }
  });
  $('insight-button').addEventListener('click', () => {
    if (!schema.some(field => field.name === 'region') || !schema.some(field => field.name === 'revenue' && String(field.kind).toLowerCase() === 'number')) {
      error('This example uses region and revenue columns. Reset the sample to explore this view.');
      return;
    }
    $('filter-enabled').checked = false;
    $('sort-enabled').checked = true;
    $('group-enabled').checked = true;
    $('group-column').value = 'region';
    $('agg-column').value = 'revenue';
    $('agg-op').value = 'sum';
    syncControls();
    $('sort-column').value = 'sum_revenue';
    $('sort-direction').value = 'desc';
    $('transform-form').requestSubmit();
  });
  $('file-input').addEventListener('change', async event => {
    const file = event.target.files[0];
    if (!file || !ready || busy) return;
    try {
      if (/\.csv$/i.test(file.name)) {
        const csv = await file.text();
        const response = await execute({ action: 'load_csv', csv }, { loadName: file.name });
        if (response) $('csv-input').value = csv;
      } else if (/\.xlsx$/i.test(file.name)) {
        const bytes = new Uint8Array(await file.arrayBuffer());
        // Keep argument lists bounded for larger workbooks.
        const chunks = [];
        for (let i = 0; i < bytes.length; i += 32768) chunks.push(String.fromCharCode(...bytes.subarray(i, i + 32768)));
        const response = await execute({ action: 'load_xlsx', data: btoa(chunks.join('')), sheet: $('sheet-name').value.trim() }, { loadName: file.name });
        if (response) $('csv-input').value = '';
      } else throw new Error('Choose a .csv or .xlsx file.');
    } catch (cause) { error(cause.message || 'The file could not be read.'); }
    finally { event.target.value = ''; }
  });

  async function download(format) {
    if (!ready || busy) return;
    busy = true;
    syncControls();
    error();
    await new Promise(resolve => requestAnimationFrame(resolve));
    try {
      const response = bridge({ action: `export_${format}` });
      if (typeof response.data !== 'string') throw new Error('The Go engine did not return an export.');
      const bytes = Uint8Array.from(atob(response.data), char => char.charCodeAt(0));
      const blob = new Blob([bytes], { type: format === 'csv' ? 'text/csv;charset=utf-8' : 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${sourceName.replace(/\.(csv|xlsx)$/i, '')}_result.${format}`;
      document.body.append(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (cause) { error(cause.message); }
    finally { busy = false; syncControls(); }
  }
  $('download-csv').addEventListener('click', () => download('csv'));
  $('download-xlsx').addEventListener('click', () => download('xlsx'));
  $('copy-install').addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText('go get github.com/Alifarid0011/sheetframe');
      $('copy-install').textContent = 'Copied!';
      setTimeout(() => { $('copy-install').textContent = 'Copy'; }, 2000);
    } catch (_) { $('copy-install').textContent = 'Select to copy'; }
  });

  async function boot() {
    $('engine-status').classList.add('loading');
    try {
      if (typeof Go !== 'function') throw new Error('The Go runtime could not be loaded. Reload the page to try again.');
      const go = new Go();
      const response = await fetch('demo.wasm');
      if (!response.ok) throw new Error(`The Go engine could not be downloaded (${response.status}). Reload the page to try again.`);
      let result;
      try { result = await WebAssembly.instantiateStreaming(response.clone(), go.importObject); }
      catch (_) { result = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject); }
      go.run(result.instance).catch(cause => {
        ready = false;
        syncControls();
        error(`The Go engine stopped: ${cause.message}`);
        $('engine-label').textContent = 'Engine unavailable';
        $('engine-status').className = 'engine-status failed';
      });
      const deadline = Date.now() + 30000;
      while (typeof window.sheetframeRun !== 'function') {
        if (Date.now() > deadline) throw new Error('The Go engine took too long to start. Reload the page to try again.');
        await new Promise(resolve => setTimeout(resolve, 40));
      }
      ready = true;
      $('engine-label').textContent = 'Go + WebAssembly · ready';
      $('engine-status').classList.remove('loading');
      syncControls();
      await loadSample();
    } catch (cause) {
      $('engine-label').textContent = 'Engine unavailable';
      $('engine-status').className = 'engine-status failed';
      $('data-table').tBodies[0].rows[0].cells[0].textContent = 'The engine is unavailable. Please reload to try again.';
      $('workspace').setAttribute('aria-busy', 'false');
      error(cause.message);
    }
  }
  syncControls();
  boot();
})();
