/**
 * feed.js — Feed loading, post cards, media carousel, likes, comments
 */

// ── Feed loading ──────────────────────────────────────────────────────────────

async function loadFeed(reset = false) {
  if (reset) S.feedPage = 1;

  const container = document.getElementById('feed-container');
  if (reset) container.innerHTML = loadingHTML();

  try {
    const data  = await apiJ(BASE.feed, `/api/v1/feed?page=${S.feedPage}&page_size=10`);
    const posts = data.posts || [];

    if (reset && !posts.length) {
      container.innerHTML = emptyHTML('Your feed is empty', 'Follow people to see their posts here');
      document.getElementById('feed-more').style.display = 'none';
      return;
    }

    if (reset) container.innerHTML = '';

    for (const post of posts) {
      container.appendChild(await buildPostCard(post));
    }

    document.getElementById('feed-more').style.display = data.has_more ? 'block' : 'none';
    S.feedPage++;
  } catch (err) {
    if (reset) container.innerHTML = emptyHTML(err.message);
  }
}

async function moreFeed() {
  await loadFeed(false);
}

// ── Post card ─────────────────────────────────────────────────────────────────

async function buildPostCard(post) {
  const card  = document.createElement('div');
  card.className = 'post-card';

  const isOwn = S.me && post.user_id == S.me.id;
  const media = post.media || [];

  if (post.username) cacheUser(post.user_id, post.username, post.avatar_url);
  const cached = getCachedUser(post.user_id);
  const uname  = cached?.username  || ('user_' + post.user_id);
  const avUrl  = cached?.avatar_url || '';

  let liked = false;
  try { liked = (await apiJ(BASE.interaction, `/api/v1/posts/${post.id}/liked`)).liked; } catch { /* ignore */ }

  card.innerHTML = `
    ${renderPostHeader(post.user_id, uname, avUrl, post.created_at, isOwn, post.id)}
    ${renderMediaCarousel(media)}
    <div class="post-actions">
      ${renderLikeButton(liked, post.like_count, post.id)}
      ${renderCommentButton(post.comment_count, post.id)}
    </div>
    ${post.caption ? `<div class="post-caption"><span class="username" onclick="openUserModal(${post.user_id},'${esc(uname)}')">${esc(uname)}</span>${esc(post.caption)}</div>` : ''}
    <div class="post-meta font-mono">${timeAgo(post.created_at)}</div>
    <div class="comments-section" id="comments-${post.id}"></div>`;

  return card;
}

function renderPostHeader(userId, username, avatarUrl, createdAt, isOwn, postId) {
  return `
    <div class="post-header">
      <div class="avatar avatar-sm" style="cursor:pointer" onclick="openUserModal(${userId},'${esc(username)}')">${avatarUrl ? `<img src="${esc(avatarUrl)}" alt="">` : esc(username[0].toUpperCase())}</div>
      <div>
        <div class="post-username" onclick="openUserModal(${userId},'${esc(username)}')">${esc(username)}</div>
        <div class="post-time font-mono">${timeAgo(createdAt)}</div>
      </div>
      ${isOwn ? `<button class="btn btn-ghost sm" style="margin-left:auto" onclick="openPostModal(${postId},true)">•••</button>` : ''}
    </div>`;
}

function renderMediaCarousel(media) {
  if (!media.length) return '';

  const first = media[0];
  const isVideo = first.media_type === 'video';
  const mainMedia = isVideo
    ? `<video src="${esc(first.media_url)}" controls style="width:100%;max-height:560px;display:block"></video>`
    : `<img src="${esc(first.media_url)}" alt="" loading="lazy">`;

  const controls = media.length > 1 ? `
    <button class="media-nav prev" onclick="mediaNav(this,-1)">&#8249;</button>
    <button class="media-nav next" onclick="mediaNav(this,1)">&#8250;</button>
    <div class="media-dots">
      ${media.map((_, i) => `<div class="media-dot${i === 0 ? ' on' : ''}"></div>`).join('')}
    </div>` : '';

  return `<div class="post-media" data-media='${JSON.stringify(media)}' data-index="0">
    ${mainMedia}
    ${controls}
  </div>`;
}

function renderLikeButton(liked, count, postId) {
  return `
    <button class="action-btn${liked ? ' liked' : ''}" onclick="toggleLike(this,${postId})">
      <svg fill="${liked ? 'currentColor' : 'none'}" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
      </svg>
      <span class="like-count">${count || 0}</span>
    </button>`;
}

function renderCommentButton(count, postId) {
  return `
    <button class="action-btn" onclick="toggleComments(this,${postId})">
      <svg fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/>
      </svg>
      <span class="comment-count">${count || 0}</span>
    </button>`;
}

// ── Media carousel navigation ─────────────────────────────────────────────────

function mediaNav(btn, dir) {
  const wrap  = btn.closest('.post-media');
  const media = JSON.parse(wrap.dataset.media);
  const next  = (parseInt(wrap.dataset.index) + dir + media.length) % media.length;
  wrap.dataset.index = next;

  const item    = media[next];
  const current = wrap.querySelector('img, video');

  if (item.media_type === 'video' && current.tagName !== 'VIDEO') {
    const v = document.createElement('video');
    v.src = item.media_url; v.controls = true;
    v.style.cssText = 'width:100%;max-height:560px;display:block';
    current.replaceWith(v);
  } else if (item.media_type !== 'video' && current.tagName !== 'IMG') {
    const img = document.createElement('img');
    img.src = item.media_url; img.loading = 'lazy';
    current.replaceWith(img);
  } else {
    current.src = item.media_url;
  }

  wrap.querySelectorAll('.media-dot').forEach((dot, i) => dot.classList.toggle('on', i === next));
}

// ── Likes ─────────────────────────────────────────────────────────────────────

async function toggleLike(btn, postId) {
  const isLiked  = btn.classList.contains('liked');
  const countEl  = btn.querySelector('.like-count');
  const svg      = btn.querySelector('svg');

  try {
    if (isLiked) {
      await api(BASE.interaction, `/api/v1/posts/${postId}/like`, { method: 'DELETE' });
      btn.classList.remove('liked');
      svg.setAttribute('fill', 'none');
      countEl.textContent = Math.max(0, parseInt(countEl.textContent) - 1);
    } else {
      await api(BASE.interaction, `/api/v1/posts/${postId}/like`, { method: 'POST' });
      btn.classList.add('liked');
      svg.setAttribute('fill', 'currentColor');
      countEl.textContent = parseInt(countEl.textContent) + 1;
    }
  } catch (err) {
    toast(err.message, 'err');
  }
}

// ── Comments ──────────────────────────────────────────────────────────────────

async function toggleComments(btn, postId) {
  const section = document.getElementById('comments-' + postId);
  if (section.classList.contains('open')) {
    section.classList.remove('open');
    return;
  }
  section.classList.add('open');
  section.innerHTML = loadingHTML();
  await renderComments(postId, section);
}

async function renderComments(postId, section) {
  try {
    const data     = await apiJ(BASE.interaction, `/api/v1/posts/${postId}/comments?page=1&page_size=20`);
    const comments = data.data || [];

    const rows = comments.map(c => {
      const uname = getCachedUser(c.user_id)?.username || c.username || ('user_' + c.user_id);
      const canDelete = S.me && c.user_id == S.me.id;
      return `
        <div class="comment-row">
          <div style="flex:1">
            <span class="username" onclick="openUserModal(${c.user_id},'${esc(uname)}')">${esc(uname)}</span>${esc(c.body)}
          </div>
          ${canDelete ? `<button class="btn btn-ghost sm" style="color:var(--red);padding:2px 6px" onclick="deleteComment(${c.id},${postId})">×</button>` : ''}
        </div>`;
    }).join('');

    section.innerHTML = rows + `
      <div class="comment-input-row">
        <input type="text" placeholder="Add a comment…" id="comment-input-${postId}"
               onkeydown="if(event.key==='Enter') addComment(${postId})">
        <button class="comment-send" onclick="addComment(${postId})">Post</button>
      </div>`;
  } catch (err) {
    section.innerHTML = `<div style="padding:12px;color:var(--text3)">${esc(err.message)}</div>`;
  }
}

async function addComment(postId) {
  const input = document.getElementById('comment-input-' + postId);
  const body  = input.value.trim();
  if (!body) return;

  try {
    await apiJ(BASE.interaction, `/api/v1/posts/${postId}/comments`, {
      method: 'POST',
      body:   JSON.stringify({ body }),
    });
    input.value = '';
    const section = document.getElementById('comments-' + postId);
    await renderComments(postId, section);
    const countEl = section.closest('.post-card')?.querySelector('.comment-count');
    if (countEl) countEl.textContent = parseInt(countEl.textContent) + 1;
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function deleteComment(commentId, postId) {
  try {
    await api(BASE.interaction, `/api/v1/comments/${commentId}`, { method: 'DELETE' });
    await renderComments(postId, document.getElementById('comments-' + postId));
  } catch (err) {
    toast(err.message, 'err');
  }
}
