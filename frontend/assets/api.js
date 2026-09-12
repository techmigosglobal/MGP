const base = '/api';
let accessToken = null;

export async function api(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (accessToken) headers.set('authorization', `Bearer ${accessToken}`);
  if (options.body && !headers.has('content-type')) headers.set('content-type', 'application/json');
  const response = await fetch(`${base}${path}`, { ...options, headers, credentials: 'include' });
  const payload = response.status === 204 ? null : await response.json().catch(() => null);
  if (!response.ok) throw new Error(payload?.errors?.[0]?.message || 'Request failed');
  return payload?.data;
}

export async function login(email, password) {
  const response = await fetch(`${base}/auth/login`, {
    method: 'POST', credentials: 'include', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ email, password, mode: 'cookie' }),
  });
  const payload = await response.json().catch(() => null);
  if (!response.ok) throw new Error(payload?.errors?.[0]?.message || 'Sign-in failed');
  accessToken = payload.data.access_token;
  return payload.data;
}

export async function restoreSession() {
  try {
    const response = await fetch(`${base}/auth/refresh`, { method: 'POST', credentials: 'include', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ mode: 'cookie' }) });
    const payload = await response.json();
    if (!response.ok) return false;
    accessToken = payload.data.access_token;
    return true;
  } catch { return false; }
}

export async function logout() {
  try { await api('/auth/logout', { method: 'POST', body: JSON.stringify({}) }); } finally { accessToken = null; }
}
