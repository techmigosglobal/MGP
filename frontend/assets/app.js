import { api, login, logout, restoreSession } from './api.js';

const app = document.querySelector('#app');
const state = { profile: null, view: 'dashboard', passes: [], inventory: [], consignees: [] };
const esc = (value = '') => String(value ?? '').replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
const date = value => value ? new Intl.DateTimeFormat('en-IN', { dateStyle: 'medium' }).format(new Date(`${value}T00:00:00`)) : '—';
const status = value => `<span class="status">${esc(String(value || '').replaceAll('_', ' '))}</span>`;
const caps = () => state.profile?.capabilities || {};

function message(text, error = false) {
  const node = document.querySelector('#message');
  if (!node) return;
  node.textContent = text;
  node.className = `notice ${error ? 'error' : ''}`;
  node.classList.remove('hidden');
}

async function loadData() {
  [state.passes, state.inventory, state.consignees] = await Promise.all([
    api('/mgp/gate-passes'), api('/mgp/inventory'), api('/mgp/consignees'),
  ]);
}

function loginView() {
  app.innerHTML = `<section class="login"><div class="login-card"><div class="brand"><strong>MGP</strong><h1>Material Gate Pass Management System</h1><p>A backend-authoritative system for controlled material movement, independent approval, security pass-out, and audit history.</p></div><form id="loginForm" class="login-form"><h2>Sign in</h2><p>Use the account provisioned by your system administrator.</p><div id="message" class="notice hidden"></div><label>Email<input name="email" type="email" autocomplete="username" required></label><label>Password<input name="password" type="password" autocomplete="current-password" required></label><button class="btn" type="submit">Sign in</button></form></div></section>`;
  document.querySelector('#loginForm').addEventListener('submit', async event => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    try { await login(data.get('email'), data.get('password')); await start(); }
    catch (error) { message(error.message, true); }
  });
}

function navigation() {
  const items = [['dashboard', 'Dashboard'], ['passes', 'Gate Pass Records']];
  if (caps().createGatePass) items.push(['create', 'Create Gate Pass']);
  items.push(['inventory', 'Inventory Master'], ['consignees', 'Consignee Master'], ['reports', 'Reports']);
  if (state.profile.user.role === 'ADMIN') items.push(['backup', 'Backup Status']);
  if (['ADMIN', 'ISSUING'].includes(state.profile.user.role)) items.push(['audit', 'Audit Log']);
  return items.map(([id, label]) => `<button data-view="${id}" class="${state.view === id ? 'active' : ''}">${label}</button>`).join('');
}

function shell(body, title, subtitle = '') {
  app.innerHTML = `<section class="shell"><aside class="sidebar"><div class="logo">MGP System</div><nav class="nav">${navigation()}</nav></aside><main class="content"><header class="top"><div><h2>${esc(title)}</h2><p>${esc(subtitle)}</p></div><div class="actions"><span class="muted">${esc(state.profile.user.name)} · ${esc(state.profile.user.role)}</span><button class="btn secondary small" data-action="logout">Sign out</button></div></header><div id="message" class="notice hidden"></div>${body}</main></section>`;
}

async function dashboard() {
  await loadData();
  const count = key => state.passes.filter(item => item.status === key).length;
  shell(`<section class="grid stats"><div class="card stat"><strong>${state.passes.length}</strong><span>Total passes</span></div><div class="card stat"><strong>${count('SUBMITTED')}</strong><span>Pending approval</span></div><div class="card stat"><strong>${count('PASSED_OUT')}</strong><span>Currently passed out</span></div><div class="card stat"><strong>${count('RETURNED')}</strong><span>Returned</span></div></section><section class="card"><h3>Recent gate passes</h3>${passesTable(state.passes.slice(0, 8))}</section>`, 'Dashboard', 'Live operational data from the MGP backend.');
}

function passActions(pass) {
  const buttons = [`<button class="btn secondary small" data-action="view-pass" data-id="${pass.id}">View</button>`];
  if (pass.status === 'DRAFT' && (state.profile.user.role === 'ADMIN' || pass.created_by === state.profile.user.id)) {
    buttons.push(`<button class="btn small" data-action="edit-pass" data-id="${pass.id}">Edit</button>`, `<button class="btn small" data-action="transition" data-transition="submit" data-id="${pass.id}">Submit</button>`);
  }
  if (pass.status === 'SUBMITTED' && caps().approve) buttons.push(`<button class="btn small" data-action="transition" data-transition="approve" data-id="${pass.id}">Approve</button>`, `<button class="btn danger small" data-action="transition" data-transition="reject" data-id="${pass.id}">Reject</button>`);
  if (pass.status === 'APPROVED' && caps().security) buttons.push(`<button class="btn warn small" data-action="transition" data-transition="pass-out" data-id="${pass.id}">Pass out</button>`);
  if (pass.status === 'PASSED_OUT' && pass.pass_type === 'RETURNABLE' && caps().security) buttons.push(`<button class="btn small" data-action="transition" data-transition="return" data-id="${pass.id}">Mark returned</button>`);
  return `<div class="actions">${buttons.join('')}</div>`;
}

function passesTable(rows) {
  if (!rows.length) return '<p class="muted">No gate passes are available.</p>';
  return `<div class="table-wrap"><table><thead><tr><th>Pass No.</th><th>Date</th><th>Consignee</th><th>Type</th><th>Status</th><th>Actions</th></tr></thead><tbody>${rows.map(pass => `<tr><td><strong>${esc(pass.pass_no)}</strong></td><td>${date(pass.pass_date)}</td><td>${esc(pass.consignee_name)}</td><td>${esc(pass.pass_type.replace('_', ' '))}</td><td>${status(pass.status)}</td><td>${passActions(pass)}</td></tr>`).join('')}</tbody></table></div>`;
}

async function passesView() {
  await loadData();
  shell(`<section class="card"><div class="top"><div><h3>Gate pass records</h3><p class="muted">Workflow actions are validated by the server.</p></div>${caps().createGatePass ? '<button class="btn" data-view="create">Create gate pass</button>' : ''}</div>${passesTable(state.passes)}</section>`, 'Gate Pass Records', 'Searchable backend records with controlled workflow actions.');
}

function itemFields(item = {}) {
  return `<div class="item-row"><input name="item_code" placeholder="Item code" value="${esc(item.item_code || '')}"><input name="item_name" placeholder="Item name" value="${esc(item.item_name || '')}" required><input name="quantity" type="number" min="0.001" step="0.001" value="${esc(item.quantity || 1)}" required><button type="button" class="btn danger small" data-action="remove-item">Remove</button></div>`;
}

function passForm(existing) {
  const p = existing || { pass_type: 'RETURNABLE', pass_date: new Date().toISOString().slice(0, 10), packages: 1, items: [{}] };
  return `<section class="card"><form id="passForm" data-id="${esc(p.id || '')}"><div class="form-grid"><label>Pass type<select name="pass_type"><option value="RETURNABLE" ${p.pass_type === 'RETURNABLE' ? 'selected' : ''}>Returnable</option><option value="NON_RETURNABLE" ${p.pass_type === 'NON_RETURNABLE' ? 'selected' : ''}>Non-returnable</option></select></label><label>Pass date<input type="date" name="pass_date" value="${esc(p.pass_date)}" required></label><label>Expected return date<input type="date" name="expected_return_date" value="${esc(p.expected_return_date || '')}"></label><label>Directorate<input name="directorate" value="${esc(p.directorate || '')}" required></label><label>Project<input name="project" value="${esc(p.project || '')}" required></label><label>Packages<input type="number" name="packages" min="1" value="${esc(p.packages)}" required></label><label>Consignee<select name="consignee_id"><option value="">Manual consignee</option>${state.consignees.map(c => `<option value="${c.id}" ${p.consignee_id === c.id ? 'selected' : ''}>${esc(c.name)}</option>`).join('')}</select></label><label>Consignee name<input name="consignee_name" value="${esc(p.consignee_name || '')}" required></label><label>Reference number<input name="reference_no" value="${esc(p.reference_no || '')}"></label><label class="full">Consignee address<textarea name="consignee_address">${esc(p.consignee_address || '')}</textarea></label><label>Inventory number<input name="inventory_no" value="${esc(p.inventory_no || '')}"></label><label>Inventory holder<input name="inventory_holder" value="${esc(p.inventory_holder || '')}"></label><label>Vehicle number<input name="vehicle_no" value="${esc(p.vehicle_no || '')}"></label><label class="full">Purpose<textarea name="purpose" required>${esc(p.purpose || '')}</textarea></label><label class="full">Authority<textarea name="authority" required>${esc(p.authority || '')}</textarea></label></div><div class="card"><div class="top"><div><h3>Material items</h3><p class="muted">Each row is saved as a normalized backend record.</p></div><button class="btn secondary small" type="button" data-action="add-item">Add item</button></div><div id="items">${(p.items || [{}]).map(itemFields).join('')}</div></div><div class="actions"><button class="btn" type="submit">${existing ? 'Save draft' : 'Create draft'}</button><button class="btn secondary" type="button" data-view="passes">Cancel</button></div></form></section>`;
}

async function createView(id = null) {
  await loadData();
  const current = id ? state.passes.find(pass => pass.id === id) : null;
  shell(passForm(current), current ? 'Edit Draft Gate Pass' : 'Create Gate Pass', 'Official pass numbers are allocated atomically by the backend.');
}

function masterView(type) {
  const inventory = type === 'inventory';
  const rows = inventory ? state.inventory : state.consignees;
  const title = inventory ? 'Inventory Master' : 'Consignee Master';
  const canManage = caps().manageMasters;
  const headers = inventory ? '<th>Code</th><th>Name</th><th>Quantity</th><th>Actions</th>' : '<th>Name</th><th>Address</th><th>Contact</th><th>Actions</th>';
  const body = rows.map(row => inventory ? `<tr><td>${esc(row.item_code)}</td><td>${esc(row.item_name)}</td><td>${esc(row.quantity)}</td><td>${canManage ? `<button class="btn secondary small" data-action="edit-master" data-type="inventory" data-id="${row.id}">Edit</button><button class="btn danger small" data-action="archive-master" data-type="inventory" data-id="${row.id}">Archive</button>` : 'Read only'}</td></tr>` : `<tr><td>${esc(row.name)}</td><td>${esc(row.address)}</td><td>${esc(row.contact)}</td><td>${canManage ? `<button class="btn secondary small" data-action="edit-master" data-type="consignees" data-id="${row.id}">Edit</button><button class="btn danger small" data-action="archive-master" data-type="consignees" data-id="${row.id}">Archive</button>` : 'Read only'}</td></tr>`).join('');
  const canExport = ['ADMIN', 'INVENTORY', 'ISSUING'].includes(state.profile.user.role);
  shell(`<section class="card"><div class="top"><div><h3>${title}</h3><p class="muted">Archived records remain traceable and are never hard deleted.</p></div><div class="actions">${canExport ? `<button class="btn secondary small" data-action="export-master" data-type="${type}">Export CSV</button>` : ''}${canManage ? `<label class="btn secondary small">Import CSV<input type="file" accept=".csv,text/csv" data-import-master="${type}" hidden></label><button class="btn" data-action="new-master" data-type="${type}">Add ${inventory ? 'item' : 'consignee'}</button>` : ''}</div></div><div id="masterForm"></div><div class="table-wrap"><table><thead><tr>${headers}</tr></thead><tbody>${body || '<tr><td colspan="4">No active records.</td></tr>'}</tbody></table></div></section>`, title, canManage ? 'Manage active master data.' : 'Read-only reference data.');
}

async function masters(type) { await loadData(); masterView(type); }

function masterForm(type, current = {}) {
  const inventory = type === 'inventory';
  return `<form id="masterFormInner" class="card" data-type="${type}" data-id="${esc(current.id || '')}"><h3>${current.id ? 'Edit' : 'Add'} ${inventory ? 'inventory item' : 'consignee'}</h3><div class="form-grid">${inventory ? `<label>Item code<input name="item_code" value="${esc(current.item_code || '')}" required></label><label>Item name<input name="item_name" value="${esc(current.item_name || '')}" required></label><label>Quantity<input name="quantity" type="number" min="0" step="0.001" value="${esc(current.quantity ?? 0)}"></label><label>Category<input name="category" value="${esc(current.category || '')}"></label><label>Serial number<input name="serial_no" value="${esc(current.serial_no || '')}"></label><label>Batch number<input name="batch_no" value="${esc(current.batch_no || '')}"></label><label>Unit<input name="unit_of_measure" value="${esc(current.unit_of_measure || 'NOS')}"></label><label>Holder<input name="holder" value="${esc(current.holder || '')}"></label><label class="full">Description<textarea name="description">${esc(current.description || '')}</textarea></label>` : `<label>Name<input name="name" value="${esc(current.name || '')}" required></label><label>Contact<input name="contact" value="${esc(current.contact || '')}"></label><label class="full">Address<textarea name="address">${esc(current.address || '')}</textarea></label>`}</div><div class="actions"><button class="btn" type="submit">Save</button><button class="btn secondary" type="button" data-action="cancel-master">Cancel</button></div></form>`;
}

function csv(rows) {
  if (!rows.length) return '';
  const keys = Object.keys(rows[0]);
  const quote = value => `"${String(value ?? '').replaceAll('"', '""')}"`;
  return [keys, ...rows.map(row => keys.map(key => row[key]))].map(row => row.map(quote).join(',')).join('\n');
}

function download(filename, content) {
  const link = document.createElement('a'); link.href = URL.createObjectURL(new Blob([content], { type: 'text/csv;charset=utf-8' })); link.download = filename; link.click(); URL.revokeObjectURL(link.href);
}

function parseCsv(text) {
  const [header, ...lines] = text.trim().split(/\r?\n/);
  if (!header) return [];
  const keys = header.split(',').map(value => value.trim());
  return lines.filter(Boolean).map(line => {
    const values = line.split(',').map(value => value.trim().replace(/^"|"$/g, '').replaceAll('""', '"'));
    return Object.fromEntries(keys.map((key, index) => [key, values[index] || '']));
  });
}

async function reportsView() {
  const rows = await api('/mgp/reports/status');
  shell(`<section class="card"><h3>Status report</h3><div class="table-wrap"><table><thead><tr><th>Status</th><th>Count</th></tr></thead><tbody>${rows.map(row => `<tr><td>${status(row.status)}</td><td>${esc(row.count)}</td></tr>`).join('') || '<tr><td colspan="2">No report data.</td></tr>'}</tbody></table></div></section>`, 'Reports', 'Server-calculated operational report.');
}

async function auditView() {
  const rows = await api('/mgp/events');
  shell(`<section class="card"><h3>Audit log</h3><div class="table-wrap"><table><thead><tr><th>Time</th><th>Action</th><th>From</th><th>To</th><th>Actor role</th><th>Reason</th></tr></thead><tbody>${rows.map(row => `<tr><td>${esc(new Date(row.created_at).toLocaleString('en-IN'))}</td><td>${esc(row.action)}</td><td>${esc(row.from_status || '—')}</td><td>${esc(row.to_status || '—')}</td><td>${esc(row.actor_role)}</td><td>${esc(row.reason || '—')}</td></tr>`).join('') || '<tr><td colspan="6">No events yet.</td></tr>'}</tbody></table></div></section>`, 'Audit Log', 'Server-generated and append-only.');
}

async function backupView() {
  const backup = await api('/mgp/backup/status');
  shell(`<section class="card"><h3>Backup status</h3><p>Last successful backup: <strong>${esc(backup.lastSuccessfulAt || 'Not recorded')}</strong></p><p class="muted">Restores are intentionally restricted to the technical host-maintenance procedure. The running application cannot replace PostgreSQL data.</p></section>`, 'Backup Status', 'MGP Admin visibility only.');
}

async function render() {
  try {
    if (state.view === 'dashboard') return dashboard();
    if (state.view === 'passes') return passesView();
    if (state.view === 'create') return createView();
    if (state.view === 'inventory' || state.view === 'consignees') return masters(state.view);
    if (state.view === 'reports') return reportsView();
    if (state.view === 'audit') return auditView();
    if (state.view === 'backup') return backupView();
    return dashboard();
  } catch (error) { shell('<section class="card"><p class="error notice">Unable to load backend data.</p></section>', 'Service unavailable'); message(error.message, true); }
}

async function start() {
  state.profile = await api('/mgp/me');
  state.view = 'dashboard';
  await render();
}

app.addEventListener('click', async event => {
  const button = event.target.closest('[data-action],[data-view]');
  if (!button) return;
  try {
    if (button.dataset.view) { state.view = button.dataset.view; await render(); return; }
    const action = button.dataset.action;
    if (action === 'logout') { await logout(); loginView(); return; }
    if (action === 'add-item') document.querySelector('#items').insertAdjacentHTML('beforeend', itemFields());
    if (action === 'remove-item') button.closest('.item-row').remove();
    if (action === 'edit-pass') await createView(button.dataset.id);
    if (action === 'view-pass') { const pass = state.passes.find(item => item.id === button.dataset.id); message(`${pass.pass_no}: ${pass.items.map(item => `${item.item_name} × ${item.quantity}`).join(', ')}`); }
    if (action === 'transition') {
      const transition = button.dataset.transition; const body = {};
      if (transition === 'reject') body.reason = window.prompt('Reason for rejection:') || '';
      if (transition === 'pass-out') body.security_control_no = window.prompt('Security control number:') || '';
      if (transition === 'return') body.actual_return_date = window.prompt('Actual return date (YYYY-MM-DD):', new Date().toISOString().slice(0, 10)) || '';
      await api(`/mgp/gate-passes/${button.dataset.id}/${transition}`, { method: 'POST', body: JSON.stringify(body) }); await passesView(); message('Workflow transition completed.');
    }
    if (action === 'archive-master') { if (!window.confirm('Archive this record? Historical gate-pass snapshots remain unchanged.')) return; await api(`/mgp/${button.dataset.type}/${button.dataset.id}/archive`, { method: 'POST' }); await masters(button.dataset.type); message('Record archived.'); }
    if (action === 'new-master') document.querySelector('#masterForm').innerHTML = masterForm(button.dataset.type);
    if (action === 'edit-master') { const rows = button.dataset.type === 'inventory' ? state.inventory : state.consignees; document.querySelector('#masterForm').innerHTML = masterForm(button.dataset.type, rows.find(row => row.id === button.dataset.id)); }
    if (action === 'cancel-master') document.querySelector('#masterForm').innerHTML = '';
    if (action === 'export-master') { const rows = await api(`/mgp/${button.dataset.type}/export`); download(`${button.dataset.type}.csv`, csv(rows)); message('CSV export downloaded.'); }
  } catch (error) { message(error.message, true); }
});

app.addEventListener('change', async event => {
  const input = event.target.closest('[data-import-master]');
  if (!input?.files?.[0]) return;
  try {
    const type = input.dataset.importMaster; const rows = parseCsv(await input.files[0].text());
    if (!rows.length) throw new Error('CSV is empty');
    for (const row of rows) await api(`/mgp/${type}`, { method: 'POST', body: JSON.stringify(row) });
    await masters(type); message(`${rows.length} record(s) imported.`);
  } catch (error) { message(error.message, true); }
});

app.addEventListener('submit', async event => {
  event.preventDefault();
  try {
    if (event.target.id === 'passForm') {
      const form = event.target; const data = Object.fromEntries(new FormData(form).entries());
      data.items = [...form.querySelectorAll('.item-row')].map(row => Object.fromEntries([...row.querySelectorAll('input')].map(input => [input.name, input.value])));
      const id = form.dataset.id; await api(id ? `/mgp/gate-passes/${id}` : '/mgp/gate-passes', { method: id ? 'PATCH' : 'POST', body: JSON.stringify(data) });
      state.view = 'passes'; await render(); message(id ? 'Draft updated.' : 'Draft created with an official pass number.');
    }
    if (event.target.id === 'masterFormInner') {
      const form = event.target; const type = form.dataset.type; const data = Object.fromEntries(new FormData(form).entries());
      const id = form.dataset.id; await api(id ? `/mgp/${type}/${id}` : `/mgp/${type}`, { method: id ? 'PATCH' : 'POST', body: JSON.stringify(data) }); await masters(type); message('Master record saved.');
    }
  } catch (error) { message(error.message, true); }
});

(async () => { if (await restoreSession()) { try { await start(); return; } catch { await logout(); } } loginView(); })();
