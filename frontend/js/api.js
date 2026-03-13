/**
 * api.js — HTTP layer
 *
 * Exposes two functions used everywhere:
 *   api(base, path, opts)  → raw Response
 *   apiJ(base, path, opts) → parsed JSON, throws on non-2xx
 *
 * All requests automatically attach the Bearer token.
 * On 401 the refresh flow runs once; if it fails, logout() is called.
 */

// Service base URLs.
// When opened directly as a file, each service is on its own port.
// When served via nginx proxy (port 3000), all requests go to the same origin.
const PROXY = location.port === '3000';

const BASE = PROXY
  ? { user: '', social: '', post: '', interaction: '', feed: '' }
  : {
      user:        'http://localhost:8080',
      social:      'http://localhost:8082',
      post:        'http://localhost:8083',
      interaction: 'http://localhost:8084',
      feed:        'http://localhost:8085',
    };

async function api(base, path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...opts.headers };

  if (S.token && !opts.noAuth) {
    headers['Authorization'] = 'Bearer ' + S.token;
  }

  let response = await fetch(base + path, { ...opts, headers });

  // Token expired — try to refresh once
  if (response.status === 401 && S.rt) {
    const refreshRes = await fetch(BASE.user + '/api/v1/auth/refresh', {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify({ refresh_token: S.rt }),
    });

    if (refreshRes.ok) {
      const tokens = await refreshRes.json();
      S.token = tokens.access_token;
      S.rt    = tokens.refresh_token;
      localStorage.setItem('tk', S.token);
      localStorage.setItem('rt', S.rt);
      headers['Authorization'] = 'Bearer ' + S.token;
      response = await fetch(base + path, { ...opts, headers });
    } else {
      logout();
      throw new Error('Session expired');
    }
  }

  return response;
}

async function apiJ(base, path, opts = {}) {
  const response = await api(base, path, opts);
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${response.status}`);
  }
  return response.json().catch(() => ({}));
}
