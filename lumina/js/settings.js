/**
 * settings.js — Settings page
 */

function loadSettings() {
  if (!S.me) return;
  document.getElementById('settings-fullname').value  = S.me.full_name || '';
  document.getElementById('settings-bio').value       = S.me.bio       || '';
  document.getElementById('settings-website').value   = S.me.website   || '';
  document.getElementById('settings-private').checked = !!S.me.is_private;
}

async function saveProfile() {
  try {
    const updated = await apiJ(BASE.user, '/api/v1/users/me', {
      method: 'PUT',
      body: JSON.stringify({
        full_name:  document.getElementById('settings-fullname').value.trim(),
        bio:        document.getElementById('settings-bio').value.trim(),
        website:    document.getElementById('settings-website').value.trim(),
        is_private: document.getElementById('settings-private').checked,
      }),
    });
    S.me = updated;
    cacheUser(updated.id, updated.username, updated.avatar_url);
    renderSidebarUser();
    toast('Profile saved', 'ok');
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function changePassword() {
  const oldPw = document.getElementById('settings-old-password').value;
  const newPw = document.getElementById('settings-new-password').value;
  if (!oldPw || !newPw) { toast('Fill in both fields', 'err'); return; }

  try {
    await apiJ(BASE.user, '/api/v1/auth/change-password', {
      method: 'PUT',
      body:   JSON.stringify({ old_password: oldPw, new_password: newPw }),
    });
    toast('Password changed — logging out', 'ok');
    setTimeout(logout, 2000);
  } catch (err) {
    toast(err.message, 'err');
  }
}

async function deleteAccount() {
  if (!confirm('Permanently delete your account? This cannot be undone.')) return;
  try {
    await api(BASE.user, '/api/v1/users/me', { method: 'DELETE' });
    logout();
  } catch (err) {
    toast(err.message, 'err');
  }
}
