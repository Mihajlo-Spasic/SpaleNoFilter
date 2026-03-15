/**
 * social.js — Search, user modal, follow/unfollow/block, list modal, follow requests
 */

// ── Search ────────────────────────────────────────────────────────────────────

function onSearchInput() {
  clearTimeout(S.searchTimer);
  S.searchTimer = setTimeout(doSearch, 320);
}

async function doSearch() {
  const query     = document.getElementById('search-input').value.trim();
  const container = document.getElementById('search-results');

  if (!query) { container.innerHTML = ''; return; }

  container.innerHTML = loadingHTML();

  try {
    const data  = await apiJ(BASE.user, `/api/v1/users/search?q=${encodeURIComponent(query)}&page_size=20`);
    const users = (data.data || []).filter(u => !S.blocked.has(u.id));

    // Cache all results — this is the main way list modals get real usernames
    users.forEach(u => cacheUser(u.id, u.username, u.avatar_url));

    if (!users.length) { container.innerHTML = emptyHTML('No users found'); return; }

    container.innerHTML = users.map(u => `
      <div class="user-row" onclick="openUserModal(${u.id},'${esc(u.username)}')">
        ${avatarHTML(u.avatar_url, u.username, 'avatar-md')}
        <div class="user-row-info">
          <div class="user-row-name">${esc(u.username)}</div>
          <div class="user-row-sub">${u.full_name ? esc(u.full_name) + ' · ' : ''}${u.follower_count || 0} followers</div>
        </div>
        ${u.is_private ? '<span class="badge badge-private">Private</span>' : ''}
      </div>`).join('');
  } catch (err) {
    container.innerHTML = emptyHTML(err.message);
  }
}

// ── User modal ────────────────────────────────────────────────────────────────

async function openUserModal(uid, knownUsername) {
  // If it's our own profile, navigate there instead
  if (S.me && uid == S.me.id) { nav('profile'); return; }

  openModal('modal-user');

  const cached      = getCachedUser(uid);
  const displayName = knownUsername || cached?.username || ('user_' + uid);
  document.getElementById('user-modal-title').textContent = displayName;

  const body = document.getElementById('user-modal-body');
  body.innerHTML = loadingHTML();

  try {
    // Fetch social status + posts concurrently; also fetch public profile if we
    // have a real username (not a "user_123" fallback).
    const requests = [
      apiJ(BASE.social, `/api/v1/social/status/${uid}`).catch(() => ({})),
      apiJ(BASE.post,   `/api/v1/users/${uid}/posts?page=1&page_size=9`).catch(() => ({ data: [] })),
    ];
    const hasRealName = displayName && !displayName.startsWith('user_');
    if (hasRealName) {
      requests.push(apiJ(BASE.user, `/api/v1/users/${encodeURIComponent(displayName)}/profile`).catch(() => null));
    }

    const [status, postsData, pub] = await Promise.all(requests);

    if (pub?.username) cacheUser(uid, pub.username, pub.avatar_url);
    const name   = pub?.username    || displayName;
    const avUrl  = pub?.avatar_url  || cached?.avatar_url || '';
    const posts  = postsData.data   || [];

    document.getElementById('user-modal-title').textContent = name;

    const { is_following, follow_status, is_blocking, is_blocked_by, is_followed_by } = status;

    body.innerHTML = `
      <div style="display:flex;gap:20px;align-items:flex-start;margin-bottom:20px">
        ${avatarHTML(avUrl, name, 'avatar-lg')}
        <div style="flex:1">
          <div style="font-family:'Cormorant Garamond',serif;font-size:22px;font-weight:300;margin-bottom:10px">${esc(name)}</div>
          ${pub?.bio ? `<div style="font-size:13px;color:var(--text2);margin-bottom:8px">${esc(pub.bio)}</div>` : ''}
          <div style="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:8px">
            ${renderFollowButton(uid, name, is_following, follow_status, is_blocking, is_blocked_by)}
            ${!is_blocking && !is_blocked_by ? `<button class="btn btn-outline sm" style="color:var(--text3)" onclick="blockUser(${uid})">Block</button>` : ''}
          </div>
          ${is_followed_by ? '<span class="badge badge-follows">Follows you</span>' : ''}
        </div>
      </div>
      ${posts.length
        ? `<hr class="divider" style="margin:0 0 16px">
           <div class="post-grid">${posts.map(p => renderGridItem(p, false)).join('')}</div>`
        : emptyHTML('No posts')}`;
  } catch (err) {
    body.innerHTML = emptyHTML(err.message);
  }
}

function renderFollowButton(uid, name, isFollowing, followStatus, isBlocking, isBlockedBy) {
  if (isBlocking)
    return `<button class="btn btn-outline sm" onclick="unblockUser(${uid},false)">Unblock</button>`;
  if (isBlockedBy)
    return `<button class="btn sm" style="background:var(--bg4);color:var(--text3);border:1px solid var(--border)" disabled>Blocked</button>`;
  if (isFollowing)
    return `<button class="btn btn-outline sm" onclick="unfollowUser(${uid},false)">Following</button>`;
  if (followStatus === 'pending')
    return `<button class="btn sm" style="background:var(--bg4);color:var(--text2);border:1px solid var(--border)" disabled>Requested</button>`;
  // Pass the name so re-opening after follow keeps the correct username
  return `<button class="btn btn-primary sm wa" onclick="followUser(${uid},'${esc(name)}')">Follow</button>`;
}

// ── Follow / unfollow / block ─────────────────────────────────────────────────

async function followUser(uid, displayName) {
  try {
    await apiJ(BASE.social, '/api/v1/social/follow', {
      method: 'POST',
      body:   JSON.stringify({ target_user_id: uid }),
    });
    toast('Follow request sent!', 'ok');
    openUserModal(uid, displayName); // re-open — name always present, no "user_XXX" regression
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function unfollowUser(uid, fromList = false) {
  try {
    await api(BASE.social, `/api/v1/social/follow/${uid}`, { method: 'DELETE' });
    toast('Unfollowed', 'ok');
    if (fromList) document.getElementById('list-row-' + uid)?.remove();
    else          openUserModal(uid, getCachedUser(uid)?.username || '');
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function blockUser(uid) {
  try {
    await apiJ(BASE.social, '/api/v1/social/block', {
      method: 'POST',
      body:   JSON.stringify({ target_user_id: uid }),
    });
    S.blocked.add(uid);
    toast('User blocked', 'ok');
    closeModal('modal-user');
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function unblockUser(uid, fromList = false) {
  try {
    await api(BASE.social, `/api/v1/social/block/${uid}`, { method: 'DELETE' });
    S.blocked.delete(uid);
    toast('Unblocked', 'ok');
    if (fromList) document.getElementById('list-row-' + uid)?.remove();
    else          openUserModal(uid, getCachedUser(uid)?.username || '');
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function removeFollower(fid) {
  try {
    await api(BASE.social, `/api/v1/social/followers/${fid}`, { method: 'DELETE' });
    document.getElementById('list-row-' + fid)?.remove();
    toast('Follower removed', 'ok');
  } catch (err) {
    toast(err.message, 'err');
  }
}

// ── Blocked cache (for search filtering) ─────────────────────────────────────

async function loadBlockedCache() {
  try {
    const data  = await apiJ(BASE.social, '/api/v1/social/blocked?page_size=50');
    S.blocked   = new Set((data.data || []).map(b => b.user_id));
  } catch {
    S.blocked = new Set();
  }
}

// ── List modal (followers / following / blocked) ──────────────────────────────

async function openListModal(type, uid, title) {
  openModal('modal-list');
  document.getElementById('list-modal-title').textContent = title;
  const body = document.getElementById('list-modal-body');
  body.innerHTML = loadingHTML();

  try {
    let items = [];
    if (type === 'followers') {
      const d = await apiJ(BASE.social, `/api/v1/social/users/${uid}/followers?page_size=50`);
      items = d.data || [];
    } else if (type === 'following') {
      const d = await apiJ(BASE.social, `/api/v1/social/users/${uid}/following?page_size=50`);
      items = d.data || [];
    } else {
      const d = await apiJ(BASE.social, '/api/v1/social/blocked?page_size=50');
      items = d.data || [];
    }

    if (!items.length) { body.innerHTML = emptyHTML('None yet'); return; }

    body.innerHTML = items.map(item => renderListRow(item, type)).join('');

    // Enrich any users not yet in cache by fetching their public profile by ID.
    // We do this after rendering so the modal appears immediately with fallback names,
    // then silently updates as profiles load.
    const unknownIds = items
      .map(item => item.user_id)
      .filter(uid => !getCachedUser(uid));

    if (unknownIds.length) {
      await Promise.allSettled(unknownIds.map(async uid => {
        try {
          const profile = await apiJ(BASE.user, `/api/v1/users/${uid}`);
          if (profile?.username) {
            cacheUser(profile.id, profile.username, profile.avatar_url);
            // Patch the rendered row in-place
            const nameEl = document.querySelector(`#list-row-${uid} .list-username`);
            const avEl   = document.querySelector(`#list-row-${uid} .list-avatar`);
            if (nameEl) nameEl.textContent = profile.username;
            if (avEl) {
              avEl.innerHTML = profile.avatar_url
                ? `<img src="${esc(profile.avatar_url)}" alt="">`
                : esc(profile.username[0].toUpperCase());
            }
          }
        } catch { /* silently ignore individual failures */ }
      }));
    }
  } catch (err) {
    body.innerHTML = emptyHTML(err.message);
  }
}

function renderListRow(item, type) {
  const uid    = item.user_id;
  const cached = getCachedUser(uid);
  const uname  = cached?.username || ('user_' + uid);
  const avUrl  = cached?.avatar_url || '';

  const actionBtn = type === 'blocked'
    ? `<button class="btn btn-outline sm" onclick="unblockUser(${uid},true)">Unblock</button>`
    : type === 'followers'
      ? `<button class="btn btn-outline sm" style="color:var(--text3)" onclick="removeFollower(${uid})">Remove</button>`
      : `<button class="btn btn-outline sm" onclick="unfollowUser(${uid},true)">Unfollow</button>`;

  // Build avatar with patchable class
  const avInner = avUrl ? `<img src="${esc(avUrl)}" alt="">` : esc(uname[0].toUpperCase());
  return `
    <div class="list-row" id="list-row-${uid}">
      <div class="avatar avatar-sm list-avatar" style="cursor:pointer" onclick="closeModal('modal-list');openUserModal(${uid},'${esc(uname)}')">${avInner}</div>
      <div style="flex:1;cursor:pointer;min-width:0" onclick="closeModal('modal-list');openUserModal(${uid},'${esc(uname)}')">
        <div style="font-weight:500;font-size:13px" class="trunc list-username">${esc(uname)}</div>
        <div style="font-size:11px;color:var(--text3)">${timeAgo(item.created_at)}</div>
      </div>
      ${actionBtn}
    </div>`;
}

// ── Follow requests ───────────────────────────────────────────────────────────

async function loadRequests() {
  const container = document.getElementById('requests-container');
  container.innerHTML = loadingHTML();

  try {
    const data  = await apiJ(BASE.social, '/api/v1/social/follow-requests/pending');
    const items = data.data || [];

    if (!items.length) { container.innerHTML = emptyHTML('No pending requests'); return; }

    container.innerHTML = items.map(item => {
      const cached = getCachedUser(item.user_id);
      const uname  = cached?.username || ('user_' + item.user_id);
      const avUrl  = cached?.avatar_url || '';
      return `
        <div class="list-row" id="request-${item.user_id}">
          <div style="cursor:pointer" onclick="openUserModal(${item.user_id},'${esc(uname)}')">
            ${avatarHTML(avUrl, uname, 'avatar-sm')}
          </div>
          <div style="flex:1">
            <div style="font-weight:500;font-size:13px">${esc(uname)}</div>
            <div style="font-size:11px;color:var(--text3)">${timeAgo(item.created_at)}</div>
          </div>
          <div style="display:flex;gap:8px">
            <button class="btn btn-primary sm wa" onclick="respondToRequest(${item.user_id},true)">Accept</button>
            <button class="btn btn-outline sm"    onclick="respondToRequest(${item.user_id},false)">Decline</button>
          </div>
        </div>`;
    }).join('');
  } catch (err) {
    container.innerHTML = emptyHTML(err.message);
  }
}

async function loadPendingBadge() {
  try {
    const data  = await apiJ(BASE.social, '/api/v1/social/follow-requests/pending');
    const count = (data.data || []).length;
    const badge = document.getElementById('notif-badge');
    if (count > 0) { badge.textContent = count; badge.style.display = 'inline'; }
    else             badge.style.display = 'none';
  } catch { /* ignore */ }
}

async function respondToRequest(followerId, accept) {
  try {
    await apiJ(BASE.social, '/api/v1/social/follow-requests/respond', {
      method: 'POST',
      body:   JSON.stringify({ follower_id: followerId, accept }),
    });
    document.getElementById('request-' + followerId)?.remove();
    toast(accept ? 'Accepted' : 'Declined', 'ok');
    loadPendingBadge();
  } catch (err) {
    toast(err.message, 'err');
  }
}
