import { execFile } from 'node:child_process';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { promisify } from 'node:util';

const exec = promisify(execFile);
const keep = process.argv.includes('--keep');
const project = `mgp-acceptance-${Date.now()}`;
const port = String(18080 + Math.floor(Math.random() * 800));
const tempDir = await mkdtemp(path.join(os.tmpdir(), 'mgp-acceptance-'));
const envPath = path.join(tempDir, '.env');
const technicalPassword = 'MgpAcceptanceTechnical-2026!';
const operatorPassword = 'MgpAcceptanceOperator-2026!';
const accounts = Object.freeze({
  ADMIN: 'admin.acceptance@example.test',
  INVENTORY: 'inventory.acceptance@example.test',
  ISSUING: 'issuing.acceptance@example.test',
  SECURITY: 'security.acceptance@example.test',
  VIEWER: 'viewer.acceptance@example.test',
});
await writeFile(envPath, [
  'POSTGRES_DB=mgp', 'POSTGRES_USER=mgp_app', 'POSTGRES_PASSWORD=mgp-acceptance-postgres-password',
  'DIRECTUS_SECRET=mgp-acceptance-directus-secret-which-is-long-enough', 'DIRECTUS_ADMIN_EMAIL=technical-admin@example.test',
  `DIRECTUS_ADMIN_PASSWORD=${technicalPassword}`, `PUBLIC_URL=http://localhost:${port}`, `MGP_WEB_PORT=${port}`, `MGP_ENV_FILE=${envPath}`,
].join('\n'));
const compose = (...args) => exec('docker', ['compose', '--project-name', project, '--env-file', envPath, '-f', 'docker-compose.yml', ...args], { cwd: process.cwd(), maxBuffer: 1024 * 1024 * 8 });
const base = `http://127.0.0.1:${port}`;
const assert = (condition, message) => { if (!condition) throw new Error(message); };

async function request(url, { token, method = 'GET', body, expected = 200 } = {}) {
  const response = await fetch(`${base}${url}`, { method, headers: { accept: 'application/json', ...(token ? { authorization: `Bearer ${token}` } : {}), ...(body ? { 'content-type': 'application/json' } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const payload = await response.json().catch(() => null);
  if (response.status !== expected) throw new Error(`${method} ${url}: expected ${expected}, received ${response.status}: ${payload?.errors?.[0]?.message || 'unexpected response'}`);
  return payload?.data;
}
async function directusLogin(email, password) { return (await request('/api/auth/login', { method: 'POST', body: { email, password, mode: 'json' } })).access_token; }
function passBody(suffix) {
  return { pass_type: 'RETURNABLE', pass_date: '2026-09-08', expected_return_date: '2026-09-12', directorate: 'ACCEPTANCE', project: 'MGP', packages: 1, consignee_name: `Acceptance consignee ${suffix}`, consignee_address: 'Acceptance yard', reference_no: `REF-${suffix}`, purpose: 'Local Docker workflow acceptance', authority: 'Acceptance supervisor', vehicle_no: 'KA-01-AC-2026', items: [{ item_code: `IT-${suffix}`, item_name: `Acceptance material ${suffix}`, quantity: 1, unit_of_measure: 'NOS' }] };
}
async function waitForStack() {
  for (let attempt = 0; attempt < 90; attempt += 1) {
    try { if ((await fetch(`${base}/api/server/health`)).ok) return; } catch { /* retry */ }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  throw new Error('Timed out waiting for Directus health through Nginx');
}

try {
  await compose('up', '--build', '-d');
  await waitForStack();
  const technicalToken = await directusLogin('technical-admin@example.test', technicalPassword);
  let roles = [];
  for (let attempt = 0; attempt < 45; attempt += 1) {
    roles = await request('/api/roles?limit=-1', { token: technicalToken });
    if (Object.values({ ADMIN: 'MGP Admin', INVENTORY: 'Inventory Holder', ISSUING: 'Issuing Officer', SECURITY: 'Security Officer', VIEWER: 'Viewer' }).every((name) => roles.some((role) => role.name === name))) break;
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  const roleNames = { ADMIN: 'MGP Admin', INVENTORY: 'Inventory Holder', ISSUING: 'Issuing Officer', SECURITY: 'Security Officer', VIEWER: 'Viewer' };
  assert(Object.values(roleNames).every((name) => roles.some((role) => role.name === name)), 'Bootstrap did not create all five MGP roles');
  for (const [code, email] of Object.entries(accounts)) {
    const roleId = roles.find((role) => role.name === roleNames[code]).id;
    await request('/api/users', { token: technicalToken, method: 'POST', expected: 200, body: { email, password: operatorPassword, role: roleId, status: 'active', first_name: `Acceptance ${code}`, last_name: 'Operator' } });
  }
  const tokens = Object.fromEntries(await Promise.all(Object.entries(accounts).map(async ([code, email]) => [code, await directusLogin(email, operatorPassword)])));
  for (const code of Object.keys(accounts)) assert((await request('/api/mgp/me', { token: tokens[code] })).user.role === code, `${code} did not receive the expected MGP role`);

  await request('/api/mgp/settings/organization', { token: tokens.ADMIN, method: 'PATCH', body: { name: 'Acceptance Materials Division', address: 'Docker validation site', defaultDirectorate: 'ACCEPTANCE', defaultProject: 'MGP' } });
  await request('/api/mgp/inventory', { token: tokens.INVENTORY, method: 'POST', body: { item_code: 'MASTER-1', item_name: 'Master validation item', quantity: 3, unit_of_measure: 'NOS' } });
  const archiveItem = await request('/api/mgp/inventory', { token: tokens.INVENTORY, method: 'POST', body: { item_code: 'ARCHIVE-1', item_name: 'Archivable item', quantity: 1, unit_of_measure: 'NOS' } });
  await request(`/api/mgp/inventory/${archiveItem.id}/archive`, { token: tokens.INVENTORY, method: 'POST', body: {} });
  assert(!(await request('/api/mgp/inventory', { token: tokens.INVENTORY })).some((item) => item.id === archiveItem.id), 'Archived inventory remained active');

  const approved = await request('/api/mgp/gate-passes', { token: tokens.INVENTORY, method: 'POST', body: passBody('APPROVE') });
  const draft = await request('/api/mgp/gate-passes', { token: tokens.INVENTORY, method: 'POST', body: passBody('DRAFT') });
  await request(`/api/mgp/gate-passes/${approved.id}/submit`, { token: tokens.INVENTORY, method: 'POST', body: {} });
  await request(`/api/mgp/gate-passes/${approved.id}/approve`, { token: tokens.INVENTORY, method: 'POST', body: {}, expected: 403 });
  await request(`/api/mgp/gate-passes/${approved.id}/approve`, { token: tokens.ISSUING, method: 'POST', body: {} });
  const securityQueue = await request('/api/mgp/gate-passes', { token: tokens.SECURITY });
  assert(securityQueue.some((pass) => pass.id === approved.id) && !securityQueue.some((pass) => pass.id === draft.id), 'Security visibility scope is incorrect');
  await request(`/api/mgp/gate-passes/${draft.id}`, { token: tokens.SECURITY, expected: 404 });
  await request(`/api/mgp/gate-passes/${approved.id}/pass-out`, { token: tokens.SECURITY, method: 'POST', body: { security_control_no: 'SEC-ACCEPT-001' } });
  const returned = await request(`/api/mgp/gate-passes/${approved.id}/return`, { token: tokens.SECURITY, method: 'POST', body: { actual_return_date: '2026-09-09' } });
  assert(returned.status === 'RETURNED', 'Return transition did not complete');

  const rejected = await request('/api/mgp/gate-passes', { token: tokens.INVENTORY, method: 'POST', body: passBody('REJECT') });
  await request(`/api/mgp/gate-passes/${rejected.id}/submit`, { token: tokens.INVENTORY, method: 'POST', body: {} });
  await request(`/api/mgp/gate-passes/${rejected.id}/reject`, { token: tokens.ISSUING, method: 'POST', body: { reason: 'Acceptance rejection evidence' } });
  const revision = await request(`/api/mgp/gate-passes/${rejected.id}/revise`, { token: tokens.INVENTORY, method: 'POST', body: {} });
  assert(revision.status === 'DRAFT' && revision.revision_of === rejected.id, 'Rejected pass revision linkage is incorrect');
  const adminDraft = await request('/api/mgp/gate-passes', { token: tokens.ADMIN, method: 'POST', body: passBody('ADMIN') });
  await request(`/api/mgp/gate-passes/${adminDraft.id}/submit`, { token: tokens.ADMIN, method: 'POST', body: {}, expected: 403 });
  const viewerPasses = await request('/api/mgp/gate-passes', { token: tokens.VIEWER });
  assert(viewerPasses.length >= 4, 'Viewer should have read-only visibility of all passes');
  await request('/api/mgp/gate-passes', { token: tokens.VIEWER, method: 'POST', body: passBody('VIEWER'), expected: 403 });
  await request('/api/mgp/events', { token: tokens.VIEWER, expected: 403 });
  await request('/api/mgp/users', { token: tokens.ADMIN });
  await request('/api/mgp/users', { token: tokens.INVENTORY, expected: 403 });
  assert((await request('/api/mgp/reports/overview', { token: tokens.VIEWER })).statuses.length > 0, 'Viewer report is empty');
  assert((await request('/api/mgp/gate-passes?status=DRAFT', { token: tokens.INVENTORY })).every((pass) => pass.status === 'DRAFT'), 'Status filtering is incorrect');
  console.log(`Docker role/API acceptance: PASS (${base})`);
  if (keep) console.log(`Browser test credentials are disposable. URL=${base} inventory=${accounts.INVENTORY}`);
} finally {
  if (!keep) {
    await compose('down', '--volumes', '--remove-orphans').catch(() => {});
    await rm(tempDir, { recursive: true, force: true });
  }
}
