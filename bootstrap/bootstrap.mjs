const baseUrl = process.env.DIRECTUS_URL;
const email = process.env.DIRECTUS_ADMIN_EMAIL;
const password = process.env.DIRECTUS_ADMIN_PASSWORD;
if (!baseUrl || !email || !password) throw new Error('DIRECTUS_URL and technical administrator credentials are required');

async function request(path, options = {}) {
  const response = await fetch(`${baseUrl}${path}`, options);
  if (!response.ok) throw new Error(`${options.method || 'GET'} ${path}: ${response.status} ${await response.text()}`);
  return response.status === 204 ? null : response.json();
}

let token;
for (let attempt = 1; attempt <= 30; attempt += 1) {
  try {
    const login = await request('/auth/login', {
      method: 'POST', headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ email, password, mode: 'json' }),
    });
    token = login.data.access_token;
    break;
  } catch (error) {
    if (attempt === 30) throw error;
    await new Promise(resolve => setTimeout(resolve, 1000));
  }
}

const headers = { authorization: `Bearer ${token}`, 'content-type': 'application/json' };
const existing = await request('/roles?limit=-1', { headers });
const names = new Set(existing.data.map(role => role.name));
for (const [name, description] of [
  ['MGP Admin', 'Constrained Material Gate Pass administrator'],
  ['Inventory Holder', 'Creates and manages own drafts and masters'],
  ['Issuing Officer', 'Approves or rejects submitted gate passes'],
  ['Security Officer', 'Records physical pass-out and returns'],
  ['Viewer', 'Read-only operational access'],
]) {
  if (names.has(name)) continue;
  await request('/roles', { method: 'POST', headers, body: JSON.stringify({ name, description, icon: 'shield', admin_access: false, app_access: false }) });
}
console.log('MGP roles bootstrapped. No application users or business records were created.');
