/**
 * auth.js — Login, register, logout, and boot sequence
 */

// ── Tab switching ─────────────────────────────────────────────────────────────

function authTab(tab) {
  const isLogin = tab === 'login';
  document.querySelectorAll('.auth-tab').forEach((el, i) => el.classList.toggle('on', i === (isLogin ? 0 : 1)));
  document.getElementById('login-form').style.display    = isLogin ? 'block' : 'none';
  document.getElementById('register-form').style.display = isLogin ? 'none'  : 'block';
}

// ── Login ─────────────────────────────────────────────────────────────────────

async function doLogin() {
  const errEl = document.getElementById('login-error');
  errEl.style.display = 'none';

  try {
    const data = await apiJ(BASE.user, '/api/v1/auth/login', {
      method:  'POST',
      noAuth:  true,
      body: JSON.stringify({
        username_or_email: document.getElementById('login-identifier').value.trim(),
        password:          document.getElementById('login-password').value,
      }),
    });
    setAuth(data);
  } catch (err) {
    errEl.textContent   = err.message;
    errEl.style.display = 'block';
  }
}

// ── Register ──────────────────────────────────────────────────────────────────

async function doRegister() {
  const errEl = document.getElementById('register-error');
  errEl.style.display = 'none';

  try {
    const data = await apiJ(BASE.user, '/api/v1/auth/register', {
      method: 'POST',
      noAuth: true,
      body: JSON.stringify({
        username:  document.getElementById('reg-username').value.trim(),
        email:     document.getElementById('reg-email').value.trim(),
        full_name: document.getElementById('reg-fullname').value.trim(),
        password:  document.getElementById('reg-password').value,
      }),
    });
    setAuth(data);
  } catch (err) {
    errEl.textContent   = err.message;
    errEl.style.display = 'block';
  }
}

// ── Session management ────────────────────────────────────────────────────────

function setAuth(data) {
  S.token = data.access_token;
  S.rt    = data.refresh_token;
  localStorage.setItem('tk', S.token);
  localStorage.setItem('rt', S.rt);
  S.me = data.user;
  if (S.me) cacheUser(S.me.id, S.me.username, S.me.avatar_url);
  boot();
}

async function doLogout() {
  try { await api(BASE.user, '/api/v1/auth/logout', { method: 'POST' }); } catch { /* ignore */ }
  logout();
}

async function doLogoutAll() {
  try { await api(BASE.user, '/api/v1/auth/logout-all', { method: 'POST' }); } catch { /* ignore */ }
  logout();
}

function logout() {
  S.token = null;
  S.rt    = null;
  S.me    = null;
  localStorage.removeItem('tk');
  localStorage.removeItem('rt');
  document.getElementById('auth').style.display = 'flex';
  document.getElementById('app').classList.remove('visible');
}

// ── Boot ──────────────────────────────────────────────────────────────────────

async function boot() {
  document.getElementById('auth').style.display = 'none';
  document.getElementById('app').classList.add('visible');

  if (!S.me) {
    try {
      S.me = await apiJ(BASE.user, '/api/v1/users/me');
    } catch {
      logout();
      return;
    }
  }

  cacheUser(S.me.id, S.me.username, S.me.avatar_url);
  renderSidebarUser();
  nav('feed');
  loadPendingBadge();
  loadBlockedCache();
}

function renderSidebarUser() {
  if (!S.me) return;
  document.getElementById('sidebar-username').textContent = S.me.username;
  const el = document.getElementById('sidebar-avatar');
  el.innerHTML = S.me.avatar_url
    ? `<img src="${esc(S.me.avatar_url)}" alt="">`
    : esc(S.me.username[0].toUpperCase());
}

// ── Navigation ────────────────────────────────────────────────────────────────

function nav(name) {
  document.querySelectorAll('.page').forEach(p => p.classList.remove('on'));
  document.querySelectorAll('.nav-item[data-page]').forEach(n => n.classList.toggle('on', n.dataset.page === name));
  document.getElementById('page-' + name)?.classList.add('on');

  if (name === 'feed')     loadFeed(true);
  if (name === 'profile')  loadProfile();
  if (name === 'notif')    loadRequests();
  if (name === 'settings') loadSettings();
  if (name === 'search')   document.getElementById('search-input').focus();
}

// ── Keyboard shortcuts for auth forms ────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('login-password').addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });
  document.getElementById('login-identifier').addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });
  document.getElementById('reg-password').addEventListener('keydown', e => { if (e.key === 'Enter') doRegister(); });

  if (S.token) boot();
});
