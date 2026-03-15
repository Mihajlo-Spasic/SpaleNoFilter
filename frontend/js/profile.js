/**
 * profile.js — Own profile page, avatar upload, post upload
 */

// ── Own profile page ──────────────────────────────────────────────────────────

async function loadProfile() {
  const container = document.getElementById('profile-container');
  container.innerHTML = loadingHTML();

  try {
    // Fetch all data in parallel. Use social-service for real follower/following
    // counts because the user-service DB columns are never updated by social-service.
    const [me, postsData, follData, follgData] = await Promise.all([
      apiJ(BASE.user,   '/api/v1/users/me'),
      apiJ(BASE.post,   `/api/v1/users/${S.me.id}/posts?page=1&page_size=24`),
      apiJ(BASE.social, `/api/v1/social/users/${S.me.id}/followers?page_size=1`).catch(() => ({ total: 0 })),
      apiJ(BASE.social, `/api/v1/social/users/${S.me.id}/following?page_size=1`).catch(() => ({ total: 0 })),
    ]);

    S.me = me;
    cacheUser(me.id, me.username, me.avatar_url);
    renderSidebarUser();

    const posts          = postsData.data || [];
    const followerCount  = follData.total  ?? 0;
    const followingCount = follgData.total ?? 0;

    container.innerHTML = `
      <div class="profile-hero">
        <div class="avatar-wrap" onclick="openAvatarModal()" title="Change photo">
          ${avatarHTML(me.avatar_url, me.username, 'avatar-lg')}
          <div class="avatar-overlay">
            <svg fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" style="width:22px;height:22px">
              <path stroke-linecap="round" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"/>
              <circle cx="12" cy="13" r="3"/>
            </svg>
            <span>Change</span>
          </div>
        </div>
        <div class="profile-info">
          <div class="profile-username-row">
            <span class="profile-username">${esc(me.username)}</span>
            ${me.is_private ? '<span class="badge badge-private">Private</span>' : ''}
          </div>
          <div class="profile-stats">
            <div class="profile-stat">
              <div class="stat-number">${me.post_count || 0}</div>
              <div class="stat-label">Posts</div>
            </div>
            <div class="profile-stat" onclick="openListModal('followers',${me.id},'Followers')">
              <div class="stat-number">${followerCount}</div>
              <div class="stat-label">Followers</div>
            </div>
            <div class="profile-stat" onclick="openListModal('following',${me.id},'Following')">
              <div class="stat-number">${followingCount}</div>
              <div class="stat-label">Following</div>
            </div>
            <div class="profile-stat" onclick="openListModal('blocked',0,'Blocked')">
              <div class="stat-number">•••</div>
              <div class="stat-label">Blocked</div>
            </div>
          </div>
          ${me.full_name ? `<div style="font-weight:500;margin-bottom:4px">${esc(me.full_name)}</div>` : ''}
          ${me.bio       ? `<div class="profile-bio">${esc(me.bio)}</div>` : ''}
          ${me.website   ? `<a class="profile-website" href="${esc(me.website)}" target="_blank">${esc(me.website)}</a>` : ''}
        </div>
      </div>
      <div class="post-grid">
        ${posts.length ? posts.map(p => renderGridItem(p, true)).join('') : emptyHTML('No posts yet')}
      </div>`;
  } catch (err) {
    container.innerHTML = emptyHTML('Failed to load profile', err.message);
  }
}

function renderGridItem(post, own = false) {
  const onclick = own ? `openPostModal(${post.id},true)` : '';
  return `
    <div class="grid-item" onclick="${onclick}">
      ${post.thumbnail_url
        ? `<img src="${esc(post.thumbnail_url)}" loading="lazy">`
        : '<div style="width:100%;height:100%;background:var(--bg4)"></div>'}
      <div class="grid-overlay">
        <span>♥ ${post.like_count || 0}</span>
        <span>💬 ${post.comment_count || 0}</span>
      </div>
    </div>`;
}

// ── Avatar upload modal ───────────────────────────────────────────────────────

function openAvatarModal() {
  S.avatarFile = null;
  document.getElementById('avatar-file-input').value = '';
  document.getElementById('avatar-preview').style.display  = 'none';
  document.getElementById('avatar-dropzone').style.display = 'block';
  document.getElementById('avatar-error').style.display    = 'none';
  openModal('modal-avatar');
}

function onAvatarFileSelected(files) {
  if (!files?.length) return;
  S.avatarFile = files[0];
  document.getElementById('avatar-preview-img').src       = URL.createObjectURL(files[0]);
  document.getElementById('avatar-preview').style.display  = 'block';
  document.getElementById('avatar-dropzone').style.display = 'none';
}

async function uploadAvatar() {
  const errEl = document.getElementById('avatar-error');
  errEl.style.display = 'none';

  if (!S.avatarFile) {
    errEl.textContent   = 'Please select a photo.';
    errEl.style.display = 'block';
    return;
  }

  const btn = document.getElementById('avatar-submit-btn');
  btn.disabled    = true;
  btn.textContent = 'Uploading…';

  try {
    // 1. Upload to dedicated avatar endpoint — no post record created, file stays in MinIO
    const fd = new FormData();
    fd.append('file', S.avatarFile);

    const uploadRes = await fetch(BASE.post + '/api/v1/upload/avatar', {
      method:  'POST',
      headers: { Authorization: 'Bearer ' + S.token },
      body:    fd,
    });
    if (!uploadRes.ok) {
      const e = await uploadRes.json();
      throw new Error(e.error || 'Upload failed');
    }
    const { url } = await uploadRes.json();
    if (!url) throw new Error('No URL returned from upload');

    // 2. Save as avatar
    await apiJ(BASE.user, '/api/v1/users/me/avatar', {
      method: 'PUT',
      body:   JSON.stringify({ avatar_url: url }),
    });

    // 3. Refresh local user data
    S.me = await apiJ(BASE.user, '/api/v1/users/me');
    cacheUser(S.me.id, S.me.username, S.me.avatar_url);
    renderSidebarUser();
    closeModal('modal-avatar');
    toast('Profile photo updated!', 'ok');
    loadProfile();
  } catch (err) {
    errEl.textContent   = err.message;
    errEl.style.display = 'block';
  } finally {
    btn.disabled    = false;
    btn.textContent = 'Upload Photo';
  }
}

// ── Post upload modal ─────────────────────────────────────────────────────────

function openUploadModal() {
  S.uploadFiles = [];
  document.getElementById('upload-preview').innerHTML  = '';
  document.getElementById('upload-caption').value      = '';
  document.getElementById('upload-error').style.display = 'none';
  document.getElementById('upload-file-input').value   = '';
  document.getElementById('upload-dropzone').className = 'upload-area';
  openModal('modal-upload');
}

function onUploadFilesSelected(files) {
  S.uploadFiles = Array.from(files);
  const preview = document.getElementById('upload-preview');
  preview.innerHTML = '';

  S.uploadFiles.forEach(file => {
    const el = file.type.startsWith('video')
      ? document.createElement('video')
      : document.createElement('img');
    el.src = URL.createObjectURL(file);
    el.style.cssText = 'width:72px;height:72px;object-fit:cover;border-radius:6px;border:1px solid var(--border)';
    if (el.tagName === 'VIDEO') { el.muted = true; el.autoplay = true; el.loop = true; }
    preview.appendChild(el);
  });

  document.getElementById('upload-dropzone').className = 'upload-area' + (files.length ? ' has-files' : '');
}

async function submitPost() {
  const errEl = document.getElementById('upload-error');
  errEl.style.display = 'none';

  if (!S.uploadFiles.length) {
    errEl.textContent   = 'Please select at least one file.';
    errEl.style.display = 'block';
    return;
  }

  const btn = document.getElementById('upload-submit-btn');
  btn.disabled    = true;
  btn.textContent = 'Uploading…';

  try {
    const fd = new FormData();
    S.uploadFiles.forEach(f => fd.append('files', f));
    const caption = document.getElementById('upload-caption').value.trim();
    if (caption) fd.append('caption', caption);

    const res = await fetch(BASE.post + '/api/v1/posts', {
      method:  'POST',
      headers: { Authorization: 'Bearer ' + S.token },
      body:    fd,
    });
    if (!res.ok) {
      const e = await res.json();
      throw new Error(e.error || 'Upload failed');
    }

    closeModal('modal-upload');
    toast('Post shared!', 'ok');
    loadFeed(true);
  } catch (err) {
    errEl.textContent   = err.message;
    errEl.style.display = 'block';
  } finally {
    btn.disabled    = false;
    btn.textContent = 'Share Post';
  }
}
