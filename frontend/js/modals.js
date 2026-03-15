/**
 * modals.js — Modal open/close, post detail modal
 */

// ── Modal open / close ────────────────────────────────────────────────────────

function openModal(id)  { document.getElementById(id).classList.add('open'); }
function closeModal(id) { document.getElementById(id).classList.remove('open'); }

// Close any modal when clicking the backdrop
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('.modal-overlay').forEach(overlay => {
    overlay.addEventListener('click', e => {
      if (e.target === overlay) overlay.classList.remove('open');
    });
  });
});

// ── Post detail modal ─────────────────────────────────────────────────────────

async function openPostModal(postId, isOwn = false) {
  S.activePid = postId;
  openModal('modal-post');

  document.getElementById('post-modal-edit-btn').style.display   = isOwn ? 'inline-flex' : 'none';
  document.getElementById('post-modal-delete-btn').style.display = isOwn ? 'inline-flex' : 'none';

  const body = document.getElementById('post-modal-body');
  body.innerHTML = loadingHTML();

  try {
    const post  = await apiJ(BASE.post, `/api/v1/posts/${postId}`);
    const media = post.media || [];

    // Build a proper carousel for multi-media posts
    let mediaHTML = '';
    if (media.length === 1) {
      const m = media[0];
      mediaHTML = m.media_type === 'video'
        ? `<video src="${esc(m.media_url)}" controls style="width:100%;border-radius:8px;margin-bottom:8px"></video>`
        : `<img src="${esc(m.media_url)}" style="width:100%;border-radius:8px;margin-bottom:8px">`;
    } else if (media.length > 1) {
      const dots = media.map((_, i) => `<div class="media-dot${i === 0 ? ' on' : ''}"></div>`).join('');
      mediaHTML = `
        <div class="post-media" data-media='${JSON.stringify(media).replace(/'/g, "&#39;")}' data-index="0" style="border-radius:8px;overflow:hidden;margin-bottom:8px">
          <img src="${esc(media[0].media_url)}" alt="" style="width:100%;display:block">
          <button class="media-nav prev" onclick="mediaNav(this,-1)">&#8249;</button>
          <button class="media-nav next" onclick="mediaNav(this,1)">&#8250;</button>
          <div class="media-dots">${dots}</div>
        </div>`;
    }

    const mediaManagerHTML = isOwn && media.length ? `
      <hr class="divider">
      <div style="font-size:11px;text-transform:uppercase;letter-spacing:1px;color:var(--text3);margin-bottom:8px">Media</div>
      <div style="display:flex;gap:8px;flex-wrap:wrap">
        ${media.map(m => `
          <div style="position:relative">
            <div style="width:60px;height:60px;border-radius:6px;border:1px solid var(--border);background:var(--bg4);overflow:hidden">
              ${m.media_type === 'image'
                ? `<img src="${esc(m.media_url)}" style="width:100%;height:100%;object-fit:cover">`
                : '<div style="display:flex;align-items:center;justify-content:center;height:100%;font-size:20px">🎬</div>'}
            </div>
            <button onclick="deleteMedia(${postId},${m.id})"
                    style="position:absolute;top:-6px;right:-6px;width:18px;height:18px;border-radius:50%;background:var(--red);border:none;color:#fff;font-size:12px;cursor:pointer;line-height:18px;text-align:center">×</button>
          </div>`).join('')}
      </div>` : '';

    body.innerHTML = `
      ${mediaHTML}
      <div id="caption-view">
        ${post.caption ? `<p style="margin-bottom:12px">${esc(post.caption)}</p>` : ''}
      </div>
      <div id="caption-edit" style="display:none">
        <div class="form-group">
          <textarea id="caption-textarea" style="height:80px">${esc(post.caption || '')}</textarea>
        </div>
        <div style="display:flex;gap:8px">
          <button class="btn btn-primary sm wa" onclick="saveCaption(${postId})">Save</button>
          <button class="btn btn-outline sm"    onclick="cancelEditCaption()">Cancel</button>
        </div>
      </div>
      <div style="font-size:12px;color:var(--text3);font-family:'DM Mono',monospace;margin-top:8px">
        ♥ ${post.like_count || 0} · 💬 ${post.comment_count || 0} · ${timeAgo(post.created_at)}
      </div>
      ${mediaManagerHTML}`;
  } catch (err) {
    body.innerHTML = emptyHTML(err.message);
  }
}

function startEditCaption() {
  document.getElementById('caption-view').style.display = 'none';
  document.getElementById('caption-edit').style.display = 'block';
}

function cancelEditCaption() {
  document.getElementById('caption-edit').style.display = 'none';
  document.getElementById('caption-view').style.display = 'block';
}

async function saveCaption(postId) {
  try {
    await apiJ(BASE.post, `/api/v1/posts/${postId}/caption`, {
      method: 'PATCH',
      body:   JSON.stringify({ caption: document.getElementById('caption-textarea').value }),
    });
    toast('Caption updated', 'ok');
    openPostModal(postId, true);
    loadFeed(true);
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function deletePost() {
  if (!confirm('Delete this post?')) return;
  try {
    await api(BASE.post, `/api/v1/posts/${S.activePid}`, { method: 'DELETE' });
    closeModal('modal-post');
    toast('Post deleted', 'ok');
    loadFeed(true);
    loadProfile();
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function deleteMedia(postId, mediaId) {
  if (!confirm('Remove this media?')) return;
  try {
    await api(BASE.post, `/api/v1/posts/${postId}/media/${mediaId}`, { method: 'DELETE' });
    toast('Media removed', 'ok');
    openPostModal(postId, true);
  } catch (err) {
    toast(err.message, 'err');
  }
}
