/**
 * state.js — Global state, user cache, toast, utilities
 *
 * S is the single source of truth for runtime state.
 * All modules read and write S directly.
 */

const S = {
  // Auth
  token:   localStorage.getItem('tk') || null,
  rt:      localStorage.getItem('rt') || null,
  me:      null,

  // Feed pagination
  feedPage: 1,

  // Upload
  uploadFiles: [],
  avatarFile:  null,

  // Active post ID (post modal)
  activePid: null,

  // Search debounce timer
  searchTimer: null,

  // Set of user IDs the current user has blocked (for search filtering)
  blocked: new Set(),

  // User cache: uid (number) → { username, avatar_url }
  userCache: {},
};

// ── User cache helpers ────────────────────────────────────────────────────────

/** Store a user's display data whenever we encounter it. */
function cacheUser(id, username, avatarUrl) {
  if (id && username) {
    S.userCache[+id] = { username, avatar_url: avatarUrl || '' };
  }
}

/** Retrieve cached user data, or null if unknown. */
function getCachedUser(id) {
  return S.userCache[+id] || null;
}

/**
 * Return the best username for a given ID.
 * Falls back to "user_<id>" when we have no data yet.
 */
function resolveUsername(id, hint) {
  return hint || getCachedUser(id)?.username || ('user_' + id);
}

// ── Toast ─────────────────────────────────────────────────────────────────────

let toastTimer;

/**
 * Show a brief notification.
 * @param {string} message
 * @param {'ok'|'err'|''} type
 */
function toast(message, type = '') {
  const el = document.getElementById('toast');
  el.textContent = message;
  el.style.color = type === 'err' ? 'var(--red)' : type === 'ok' ? 'var(--green)' : 'var(--text)';
  el.style.opacity = '1';
  el.style.transform = 'translateX(-50%) translateY(0)';
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { el.style.opacity = '0'; }, 3200);
}

// ── HTML helpers ──────────────────────────────────────────────────────────────

/** Escape a value for safe insertion into HTML. */
function esc(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

/** Format an ISO timestamp into a human-readable relative time. */
function timeAgo(iso) {
  if (!iso) return '';
  const d    = new Date(iso);
  const secs = (Date.now() - d) / 1000;
  if (secs < 60)     return 'just now';
  if (secs < 3600)   return Math.floor(secs / 60)   + 'm ago';
  if (secs < 86400)  return Math.floor(secs / 3600)  + 'h ago';
  if (secs < 604800) return Math.floor(secs / 86400) + 'd ago';
  return d.toLocaleDateString('en', { month: 'short', day: 'numeric' });
}

/** Render a standard spinner + loading message. */
function loadingHTML(msg = 'Loading…') {
  return `<div class="loading"><div class="spinner"></div>${esc(msg)}</div>`;
}

/** Render an empty-state block with an optional icon. */
function emptyHTML(message, sub = '') {
  return `<div class="empty-state">
    <p>${esc(message)}</p>
    ${sub ? `<small>${esc(sub)}</small>` : ''}
  </div>`;
}

/**
 * Render an avatar element.
 * @param {string|null} url
 * @param {string}      fallback  — single character shown when no image
 * @param {string}      sizeClass — avatar-sm | avatar-md | avatar-lg
 */
function avatarHTML(url, fallback, sizeClass = 'avatar-sm') {
  const inner = url ? `<img src="${esc(url)}" alt="">` : esc(fallback.toUpperCase()[0]);
  return `<div class="avatar ${sizeClass}">${inner}</div>`;
}

/** Standard SVG close icon used in modals. */
const ICON_CLOSE = `<svg fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" style="width:20px;height:20px"><path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/></svg>`;
