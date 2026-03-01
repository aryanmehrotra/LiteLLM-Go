// LLM Gateway Admin Dashboard

const API = '';
let masterKey = sessionStorage.getItem('gw_master_key') || '';
let userRole = sessionStorage.getItem('gw_role') || '';

// ===================== TOAST =====================
function toast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  const icons = {
    success: '<svg viewBox="0 0 24 24" fill="none" stroke="#34d399" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
    error: '<svg viewBox="0 0 24 24" fill="none" stroke="#f87171" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>',
    info: '<svg viewBox="0 0 24 24" fill="none" stroke="#0ea5e9" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>'
  };
  const el = document.createElement('div');
  el.className = 'toast toast-' + type;
  el.innerHTML = '<div class="toast-icon">' + icons[type] + '</div><div class="toast-message">' + esc(message) + '</div>';
  container.appendChild(el);
  setTimeout(() => {
    el.style.opacity = '0';
    el.style.transform = 'translateX(32px)';
    el.style.transition = 'all 0.3s ease';
    setTimeout(() => el.remove(), 300);
  }, 4000);
}

// ===================== AUTH =====================
function getHeaders() {
  return { 'Authorization': 'Bearer ' + masterKey, 'Content-Type': 'application/json' };
}

async function apiFetch(path, opts = {}) {
  const resp = await fetch(API + path, { headers: getHeaders(), ...opts });
  if (resp.status === 401) {
    sessionStorage.removeItem('gw_master_key');
    sessionStorage.removeItem('gw_role');
    masterKey = '';
    userRole = '';
    showLogin();
    throw new Error('Unauthorized');
  }
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({}));
    throw new Error(err.error?.message || err.error || resp.statusText);
  }
  if (resp.status === 204 || resp.headers.get('content-length') === '0') {
    return {};
  }
  const data = await resp.json().catch(() => ({}));
  return data || {};
}

function isAdmin() { return userRole === 'admin'; }

// ===================== NAVIGATION =====================
const adminPages = ['dashboard', 'models', 'keys', 'spend', 'batches', 'access', 'guardrails', 'audit', 'settings', 'playground', 'docs'];
const userPages = ['dashboard', 'mykey', 'myspend', 'playground', 'docs'];
const allPageIds = [...new Set([...adminPages, ...userPages])];

const pageLabels = {
  dashboard: 'Dashboard', models: 'Models', keys: 'Virtual Keys', spend: 'Spend & Usage',
  batches: 'Batch Jobs', access: 'Access Control',
  guardrails: 'Guardrails', audit: 'Audit Log', settings: 'Settings',
  mykey: 'My Key', myspend: 'My Usage', playground: 'Playground', docs: 'API Docs'
};

function navigate(page) {
  allPageIds.forEach(p => {
    const pageEl = document.getElementById('page-' + p);
    if (pageEl) pageEl.classList.add('hidden');
    document.querySelectorAll('[id="nav-' + p + '"]').forEach(el => el.classList.remove('active'));
  });
  const pageEl = document.getElementById('page-' + page);
  if (pageEl) pageEl.classList.remove('hidden');
  document.querySelectorAll('[id="nav-' + page + '"]').forEach(el => el.classList.add('active'));
  document.getElementById('topbar-page').textContent = pageLabels[page] || page;

  const loaders = {
    dashboard: isAdmin() ? loadDashboard : loadUserDashboard,
    models: loadModels, keys: loadKeys, spend: loadSpend,
    batches: loadBatches, access: loadAccess,
    guardrails: loadGuardrails, audit: loadAudit, settings: loadSettings,
    mykey: loadMyKey, myspend: loadMySpend, playground: loadPlayground, docs: loadDocs
  };
  if (loaders[page]) loaders[page]();
}

// ===================== ROLE-BASED UI =====================
function applyRole() {
  // Show/hide sidebar sections
  document.querySelectorAll('[data-role]').forEach(el => {
    const roles = el.dataset.role.split(',');
    el.style.display = roles.includes(userRole) ? '' : 'none';
  });

  // Update topbar user info
  document.getElementById('topbar-role-badge').textContent = isAdmin() ? 'Admin' : 'User';
  document.getElementById('topbar-role-badge').className = 'topbar-role ' + (isAdmin() ? 'role-admin' : 'role-user');
  document.getElementById('topbar-avatar').textContent = isAdmin() ? 'A' : 'U';
  document.querySelector('.topbar-user span').textContent = isAdmin() ? 'Admin' : 'Key Holder';
}

// ===================== LOGIN / LOGOUT =====================
function showLogin() {
  document.getElementById('app').style.display = 'none';
  document.getElementById('login').style.display = '';
  document.getElementById('login-key').value = '';
  document.getElementById('login-error').style.display = 'none';
}

function showApp() {
  document.getElementById('login').style.display = 'none';
  document.getElementById('app').style.display = 'contents';
  applyRole();
  navigate('dashboard');
}

async function doLogin() {
  const key = document.getElementById('login-key').value.trim();
  if (!key) return;
  masterKey = key;

  try {
    // Verify key and determine role
    const authResp = await apiFetch('/auth/check');
    const data = authResp.data || authResp;
    userRole = data.role || 'user';
    sessionStorage.setItem('gw_master_key', key);
    sessionStorage.setItem('gw_role', userRole);
    document.getElementById('login-error').style.display = 'none';
    showApp();
    toast(isAdmin() ? 'Welcome back, Admin!' : 'Welcome! Viewing your key dashboard.', 'success');
  } catch {
    document.getElementById('login-error').style.display = 'block';
    masterKey = '';
    userRole = '';
  }
}

function doLogout() {
  sessionStorage.removeItem('gw_master_key');
  sessionStorage.removeItem('gw_role');
  masterKey = '';
  userRole = '';
  showLogin();
  toast('Logged out successfully', 'info');
}

// ===================== ADMIN DASHBOARD =====================
async function loadDashboard() {
  try {
    const health = await apiFetch('/health');
    const d = health.data || health;
    document.getElementById('stat-models').textContent = d.models || 0;
    document.getElementById('stat-status').textContent = d.status || 'healthy';
  } catch {
    document.getElementById('stat-status').textContent = 'error';
    document.getElementById('stat-status').className = 'value danger';
  }
  try {
    const keys = await apiFetch('/keys');
    const d = keys.data || keys || [];
    document.getElementById('stat-keys').textContent = Array.isArray(d) ? d.length : 0;
  } catch { document.getElementById('stat-keys').textContent = '?'; }
  try {
    const batches = await apiFetch('/v1/batches?limit=100');
    const d = batches.data || batches || {};
    const list = d.data || [];
    document.getElementById('stat-batches').textContent = Array.isArray(list) ? list.length : '0';
  } catch { document.getElementById('stat-batches').textContent = '?'; }
  try {
    const t = await apiFetch('/teams');
    const td = t.data || t || [];
    document.getElementById('info-teams').textContent = Array.isArray(td) ? td.length : '0';
  } catch { document.getElementById('info-teams').textContent = '?'; }
  try {
    const u = await apiFetch('/users');
    const ud = u.data || u || [];
    document.getElementById('info-users').textContent = Array.isArray(ud) ? ud.length : '0';
  } catch { document.getElementById('info-users').textContent = '?'; }
  try {
    const resp = await apiFetch('/spend/report?group_by=model');
    const data = resp.data || resp || [];
    const activityEl = document.getElementById('dashboard-activity');
    if (Array.isArray(data) && data.length > 0) {
      activityEl.innerHTML = data.slice(0, 8).map(r => `
        <div class="activity-item">
          <div class="activity-dot accent"></div>
          <div class="activity-text"><strong>${esc(r.group_value || r.group_by || 'unknown')}</strong> &mdash; ${r.request_count || r.total_requests || 0} requests</div>
          <div class="activity-time">$${(r.total_cost || 0).toFixed(4)}</div>
        </div>
      `).join('');
    }
  } catch { /* silent */ }
}

// ===================== USER DASHBOARD (virtual key holder) =====================
async function loadUserDashboard() {
  const statsEl = document.getElementById('user-dash-stats');
  const infoEl = document.getElementById('user-dash-info');
  const spendEl = document.getElementById('user-dash-spend');

  // Load own key info
  try {
    const resp = await apiFetch('/key/self');
    const k = resp.data || resp;
    statsEl.innerHTML = `
      <div class="stat-card">
        <div class="stat-card-header"><div class="label">Key Name</div><div class="stat-card-icon accent"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg></div>
        </div>
        <div class="value accent" style="font-size:22px">${esc(k.name)}</div>
        <div class="stat-footer"><code>${esc(k.key_prefix)}...</code></div>
      </div>
      <div class="stat-card">
        <div class="stat-card-header"><div class="label">Tier</div><div class="stat-card-icon purple"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg></div>
        </div>
        <div class="value purple">${esc(k.tier || 'default')}</div>
        <div class="stat-footer">Access tier</div>
      </div>
      <div class="stat-card">
        <div class="stat-card-header"><div class="label">RPM Limit</div><div class="stat-card-icon warning"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></div>
        </div>
        <div class="value warning">${k.rate_limit_rpm || '&#8734;'}</div>
        <div class="stat-footer">Requests per minute</div>
      </div>
      <div class="stat-card">
        <div class="stat-card-header"><div class="label">Budget</div><div class="stat-card-icon success"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="1" x2="12" y2="23"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg></div>
        </div>
        <div class="value success">${k.max_budget ? '$' + k.max_budget.toFixed(2) : '&#8734;'}</div>
        <div class="stat-footer">Maximum budget</div>
      </div>
    `;

    // Key details
    infoEl.innerHTML = `
      <div class="activity-item"><div class="activity-dot ${k.is_active ? 'success' : 'danger'}"></div><div class="activity-text">Status</div><div class="activity-time"><span class="badge ${k.is_active ? 'badge-active' : 'badge-inactive'}">${k.is_active ? 'Active' : 'Inactive'}</span></div></div>
      <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Team</div><div class="activity-time">${esc(k.team_id || '—')}</div></div>
      <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Organization</div><div class="activity-time">${esc(k.org_id || '—')}</div></div>
      <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Allowed Models</div><div class="activity-time">${esc(k.allowed_models || 'All')}</div></div>
      <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">TPM Limit</div><div class="activity-time">${k.rate_limit_tpm || '&#8734;'}</div></div>
      <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Expires</div><div class="activity-time">${k.expires_at ? new Date(k.expires_at).toLocaleDateString() : 'Never'}</div></div>
      <div class="activity-item"><div class="activity-dot success"></div><div class="activity-text">Created</div><div class="activity-time">${k.created_at ? new Date(k.created_at).toLocaleDateString() : '—'}</div></div>
    `;
  } catch (e) {
    statsEl.innerHTML = '<div class="stat-card"><div class="value danger">Error loading key info</div></div>';
  }

  // Load own spend
  try {
    const resp = await apiFetch('/spend/self?group_by=model');
    const data = resp.data || resp.data || [];
    const rows = Array.isArray(data) ? data : [];
    if (rows.length > 0) {
      let totalCost = 0;
      rows.forEach(r => totalCost += r.total_cost || 0);
      spendEl.innerHTML = `
        <div class="card-header"><h3>My Usage</h3><span style="color:var(--success);font-weight:600">$${totalCost.toFixed(4)} total</span></div>
        <div class="card-body no-pad">
          <table><thead><tr><th>Model</th><th>Requests</th><th>Tokens</th><th>Cost</th></tr></thead>
          <tbody>${rows.map(r => `<tr>
            <td style="font-weight:500;color:var(--text-primary)">${esc(r.group_by || '—')}</td>
            <td>${(r.request_count || 0).toLocaleString()}</td>
            <td>${(r.total_tokens || 0).toLocaleString()}</td>
            <td style="color:var(--success);font-weight:500">$${(r.total_cost || 0).toFixed(4)}</td>
          </tr>`).join('')}</tbody></table>
        </div>`;
    } else {
      spendEl.innerHTML = `<div class="card-header"><h3>My Usage</h3></div><div class="card-body"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="1" x2="12" y2="23"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg></div><h4>No usage yet</h4><p>Start making API calls to see your spend here.</p></div></div>`;
    }
  } catch { spendEl.innerHTML = '<div class="card-header"><h3>My Usage</h3></div><div class="card-body"><p class="text-muted">Unable to load spend data.</p></div>'; }
}

// ===================== MY KEY (user page) =====================
async function loadMyKey() {
  const container = document.getElementById('mykey-content');
  try {
    const resp = await apiFetch('/key/self');
    const k = resp.data || resp;
    container.innerHTML = `
      <div class="card"><div class="card-header"><h3>Key Details</h3><span class="badge ${k.is_active ? 'badge-active' : 'badge-inactive'}">${k.is_active ? 'Active' : 'Inactive'}</span></div>
      <div class="card-body">
        <div class="detail-grid">
          <div class="detail-item"><div class="detail-label">Key Prefix</div><div class="detail-value"><code>${esc(k.key_prefix)}...</code></div></div>
          <div class="detail-item"><div class="detail-label">Name</div><div class="detail-value">${esc(k.name)}</div></div>
          <div class="detail-item"><div class="detail-label">Tier</div><div class="detail-value">${esc(k.tier || 'default')}</div></div>
          <div class="detail-item"><div class="detail-label">Team</div><div class="detail-value">${esc(k.team_id || '—')}</div></div>
          <div class="detail-item"><div class="detail-label">Organization</div><div class="detail-value">${esc(k.org_id || '—')}</div></div>
          <div class="detail-item"><div class="detail-label">User</div><div class="detail-value">${esc(k.user_id || '—')}</div></div>
          <div class="detail-item"><div class="detail-label">RPM Limit</div><div class="detail-value">${k.rate_limit_rpm || 'Unlimited'}</div></div>
          <div class="detail-item"><div class="detail-label">TPM Limit</div><div class="detail-value">${k.rate_limit_tpm || 'Unlimited'}</div></div>
          <div class="detail-item"><div class="detail-label">Max Budget</div><div class="detail-value">${k.max_budget ? '$' + k.max_budget.toFixed(2) : 'Unlimited'}</div></div>
          <div class="detail-item"><div class="detail-label">Allowed Models</div><div class="detail-value">${esc(k.allowed_models || 'All models')}</div></div>
          <div class="detail-item"><div class="detail-label">Expires</div><div class="detail-value">${k.expires_at ? new Date(k.expires_at).toLocaleString() : 'Never'}</div></div>
          <div class="detail-item"><div class="detail-label">Created</div><div class="detail-value">${k.created_at ? new Date(k.created_at).toLocaleString() : '—'}</div></div>
        </div>
      </div></div>`;
  } catch (e) {
    container.innerHTML = '<div class="card"><div class="card-body"><p class="text-muted">Unable to load key info.</p></div></div>';
    toast('Failed to load key info: ' + e.message, 'error');
  }
}

// ===================== MY SPEND (user page) =====================
async function loadMySpend() {
  const groupBy = document.getElementById('myspend-group-by').value;
  const colLabel = { model: 'Model', provider: 'Provider' }[groupBy] || 'Model';
  document.getElementById('myspend-col-group').textContent = colLabel;

  const tbody = document.getElementById('myspend-tbody');
  try {
    const resp = await apiFetch('/spend/self?group_by=' + groupBy);
    const data = resp.data || [];
    const rows = Array.isArray(data) ? data : [];

    if (rows.length === 0) {
      document.getElementById('myspend-total').textContent = '$0.00';
      document.getElementById('myspend-requests').textContent = '0';
      document.getElementById('myspend-tokens').textContent = '0';
      tbody.innerHTML = '<tr><td colspan="4"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="1" x2="12" y2="23"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg></div><h4>No usage data</h4><p>Make API calls with your key to see usage here.</p></div></td></tr>';
      return;
    }

    let totalCost = 0, totalReqs = 0, totalTokens = 0;
    rows.forEach(r => { totalCost += r.total_cost || 0; totalReqs += r.request_count || 0; totalTokens += r.total_tokens || 0; });
    document.getElementById('myspend-total').textContent = '$' + totalCost.toFixed(4);
    document.getElementById('myspend-requests').textContent = totalReqs.toLocaleString();
    document.getElementById('myspend-tokens').textContent = totalTokens.toLocaleString();

    tbody.innerHTML = rows.map(r => `<tr>
      <td style="font-weight:500;color:var(--text-primary)">${esc(r.group_by || '—')}</td>
      <td>${(r.request_count || 0).toLocaleString()}</td>
      <td>${(r.total_tokens || 0).toLocaleString()}</td>
      <td style="color:var(--success);font-weight:500">$${(r.total_cost || 0).toFixed(4)}</td>
    </tr>`).join('');
  } catch (e) { toast('Failed to load spend data: ' + e.message, 'error'); }
}

// ===================== MODELS =====================
let allModels = [];

async function loadModels() {
  const tbody = document.getElementById('models-tbody');
  try {
    const resp = await apiFetch('/v1/models');
    const data = resp.data || resp || [];
    const models = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
    allModels = models;
    // Build provider stats as an ordered array (preserves API response order)
    const providerStats = [];
    const providerIndex = {};
    models.forEach(m => {
      const p = (m.id || '').split('/')[0] || m.provider || 'unknown';
      if (!(p in providerIndex)) { providerIndex[p] = providerStats.length; providerStats.push({ name: p, count: 0 }); }
      providerStats[providerIndex[p]].count++;
    });
    const statsEl = document.getElementById('models-provider-stats');
    const colors = ['accent', 'success', 'purple', 'warning', 'danger'];
    statsEl.innerHTML = providerStats.map((ps, i) => {
      const color = colors[i % colors.length];
      return `<div class="stat-card"><div class="stat-card-header"><div class="label">${esc(ps.name)}</div></div><div class="value ${color}">${ps.count}</div><div class="stat-footer">models</div></div>`;
    }).join('');
    renderModels(models);
  } catch (e) { toast('Failed to load models: ' + e.message, 'error'); }
}

function renderModels(models) {
  const tbody = document.getElementById('models-tbody');
  if (!models.length) {
    tbody.innerHTML = '<tr><td colspan="3"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/></svg></div><h4>No models</h4><p>Configure providers to see available models.</p></div></td></tr>';
    return;
  }
  tbody.innerHTML = models.map(m => {
    const id = m.id || '';
    const parts = id.split('/');
    const provider = parts.length > 1 ? parts[0] : (m.provider || 'unknown');
    const type = m.type === 'embedding' ? 'Embedding' : m.type === 'chat' ? 'Chat' : id.includes('embed') ? 'Embedding' : 'Chat';
    const typeCls = type === 'Embedding' ? 'badge-pending' : 'badge-active';
    return `<tr><td><code>${esc(id)}</code></td><td><span class="badge badge-processing">${esc(provider)}</span></td><td><span class="badge ${typeCls}">${type}</span></td></tr>`;
  }).join('');
}

// ===================== KEYS =====================
async function loadKeys() {
  const tbody = document.getElementById('keys-tbody');
  try {
    const resp = await apiFetch('/keys');
    const keys = resp.data || resp || [];
    if (!Array.isArray(keys) || keys.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg></div><h4>No virtual keys</h4><p>Generate your first API key to get started.</p></div></td></tr>';
      return;
    }
    tbody.innerHTML = keys.map(k => `
      <tr>
        <td><code>${esc(k.key_prefix || '')}...</code></td>
        <td style="font-weight:500;color:var(--text-primary)">${esc(k.name)}</td>
        <td>${esc(k.team_id || '—')}</td>
        <td>${esc(k.tier || 'default')}</td>
        <td>${k.rate_limit_rpm || '&#8734;'}</td>
        <td><span class="badge ${k.is_active ? 'badge-active' : 'badge-inactive'}">${k.is_active ? 'Active' : 'Inactive'}</span></td>
        <td style="text-align:right;white-space:nowrap">
          <button class="btn btn-secondary btn-sm" onclick="rotateKey(${k.id})" title="Rotate key"><svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg> Rotate</button>
          <button class="btn btn-danger btn-sm" onclick="deleteKey(${k.id})"><svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg> Delete</button>
        </td>
      </tr>
    `).join('');
  } catch (e) { toast('Failed to load keys: ' + e.message, 'error'); }
}

async function showGenerateKeyModal() {
  ['gk-name','gk-tier','gk-rpm','gk-budget','gk-models','gk-expires'].forEach(id => document.getElementById(id).value = '');
  document.getElementById('gk-team').value = '';
  if (document.getElementById('gk-org')) document.getElementById('gk-org').value = '';
  if (document.getElementById('gk-user')) document.getElementById('gk-user').value = '';
  await populateDropdowns();
  document.getElementById('generate-key-modal').classList.remove('hidden');
  setTimeout(() => document.getElementById('gk-name').focus(), 100);
}

function hideGenerateKeyModal() { document.getElementById('generate-key-modal').classList.add('hidden'); }

async function generateKey() {
  const name = document.getElementById('gk-name').value.trim();
  if (!name) { toast('Key name is required', 'error'); return; }
  const orgId = document.getElementById('gk-org') ? document.getElementById('gk-org').value : '';
  if (!orgId) { toast('Organization is required', 'error'); return; }
  const models = document.getElementById('gk-models').value.trim();
  const body = {
    name, org_id: orgId,
    team_id: document.getElementById('gk-team').value || undefined,
    user_id: document.getElementById('gk-user') ? (document.getElementById('gk-user').value || undefined) : undefined,
    tier: document.getElementById('gk-tier').value || undefined,
    rate_limit_rpm: parseInt(document.getElementById('gk-rpm').value) || 0,
    max_budget: parseFloat(document.getElementById('gk-budget').value) || 0,
    allowed_models: models ? models.split(',').map(s => s.trim()) : undefined,
    expires_in_days: parseInt(document.getElementById('gk-expires').value) || 0,
  };
  try {
    const resp = await apiFetch('/key/generate', { method: 'POST', body: JSON.stringify(body) });
    const data = resp.data || resp;
    hideGenerateKeyModal();
    document.getElementById('key-reveal-value').textContent = data.key || 'unknown';
    document.getElementById('key-reveal-modal').classList.remove('hidden');
    toast('Virtual key generated successfully', 'success');
    loadKeys();
  } catch (e) { toast('Failed to generate key: ' + e.message, 'error'); }
}

function copyKey() {
  const key = document.getElementById('key-reveal-value').textContent;
  navigator.clipboard.writeText(key).then(() => toast('Key copied to clipboard', 'success'))
    .catch(() => { const ta = document.createElement('textarea'); ta.value = key; document.body.appendChild(ta); ta.select(); document.execCommand('copy'); document.body.removeChild(ta); toast('Key copied to clipboard', 'success'); });
}

async function rotateKey(id) {
  if (!confirm('Rotate this key? The old key will be deactivated and a new one generated.')) return;
  try {
    const resp = await apiFetch('/key/' + id + '/rotate', { method: 'POST' });
    const data = resp.data || resp;
    document.getElementById('key-reveal-value').textContent = data.key || 'unknown';
    document.getElementById('key-reveal-modal').classList.remove('hidden');
    toast('Key rotated successfully', 'success');
    loadKeys();
  } catch (e) { toast('Failed to rotate key: ' + e.message, 'error'); }
}

async function deleteKey(id) {
  if (!confirm('Delete this key? This action cannot be undone.')) return;
  try { await apiFetch('/key/' + id, { method: 'DELETE' }); toast('Key deleted', 'success'); loadKeys(); }
  catch (e) { toast('Failed to delete key: ' + e.message, 'error'); }
}

// ===================== SPEND (admin) =====================
async function loadSpend() {
  const groupBy = document.getElementById('spend-group-by').value;
  const colLabel = { model: 'Model', provider: 'Provider', key: 'Key', team: 'Team', user: 'User', org: 'Organization' }[groupBy] || 'Model';
  document.getElementById('spend-col-group').textContent = colLabel;
  document.getElementById('spend-table-title').textContent = 'Usage by ' + colLabel;
  const tbody = document.getElementById('spend-tbody');
  try {
    const resp = await apiFetch('/spend/report?group_by=' + groupBy);
    const rawData = resp.data || resp || {};
    const data = Array.isArray(rawData) ? rawData : (rawData.data || []);
    if (!Array.isArray(data) || data.length === 0) {
      document.getElementById('spend-total').textContent = '$0.00';
      document.getElementById('spend-requests').textContent = '0';
      document.getElementById('spend-tokens').textContent = '0';
      tbody.innerHTML = '<tr><td colspan="5"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="1" x2="12" y2="23"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg></div><h4>No spend data</h4><p>Usage data will appear here once requests are processed.</p></div></td></tr>';
      return;
    }
    let totalCost = 0, totalReqs = 0, totalTokens = 0;
    data.forEach(r => { totalCost += r.total_cost || 0; totalReqs += r.request_count || r.total_requests || 0; totalTokens += r.total_tokens || 0; });
    document.getElementById('spend-total').textContent = '$' + totalCost.toFixed(4);
    document.getElementById('spend-requests').textContent = totalReqs.toLocaleString();
    document.getElementById('spend-tokens').textContent = totalTokens.toLocaleString();
    tbody.innerHTML = data.map(r => `<tr>
      <td style="font-weight:500;color:var(--text-primary)">${esc(r.group_by || r.group_value || '—')}</td>
      <td>${(r.request_count || r.total_requests || 0).toLocaleString()}</td>
      <td>${(r.total_tokens || 0).toLocaleString()}</td>
      <td style="color:var(--success);font-weight:500">$${(r.total_cost || 0).toFixed(4)}</td>
      <td class="text-sm text-muted">${esc(r.period || '—')}</td>
    </tr>`).join('');
  } catch (e) { toast('Failed to load spend data: ' + e.message, 'error'); }
}

// ===================== BATCHES =====================
async function loadBatches() {
  const tbody = document.getElementById('batches-tbody');
  try {
    const resp = await apiFetch('/v1/batches?limit=50');
    const data = resp.data || resp || {};
    const batches = data.data || [];
    if (!Array.isArray(batches) || batches.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></div><h4>No batch jobs</h4><p>Submit a batch via the API to see jobs here.</p></div></td></tr>';
      return;
    }
    tbody.innerHTML = batches.map(b => {
      const total = b.total_requests || 0;
      const done = (b.completed_requests || 0) + (b.failed_requests || 0);
      const progress = total > 0 ? Math.round(done / total * 100) : 0;
      const cls = b.status === 'completed' ? 'complete' : b.status === 'failed' ? 'failed' : '';
      const created = b.created_at ? new Date(b.created_at).toLocaleString() : '—';
      return `<tr>
        <td><code class="text-sm">${esc(b.id)}</code></td>
        <td><span class="badge badge-${b.status}">${b.status}</span></td>
        <td>${total}</td>
        <td style="min-width:160px"><div style="display:flex;align-items:center;gap:10px"><div class="progress-bar" style="flex:1"><div class="progress-bar-fill ${cls}" style="width:${progress}%"></div></div><span class="text-sm text-muted" style="min-width:40px">${progress}%</span></div><div class="text-sm text-muted mt-8">${b.completed_requests || 0} done, ${b.failed_requests || 0} failed</div></td>
        <td class="text-sm text-muted">${created}</td>
        <td style="text-align:right">${b.status === 'pending' || b.status === 'processing' ? `<button class="btn btn-danger btn-sm" onclick="cancelBatch('${esc(b.id)}')">Cancel</button>` : b.status === 'completed' ? `<button class="btn btn-secondary btn-sm" onclick="viewResults('${esc(b.id)}')">Results</button>` : ''}</td>
      </tr>`;
    }).join('');
  } catch (e) { toast('Failed to load batches: ' + e.message, 'error'); }
}

async function cancelBatch(id) {
  if (!confirm('Cancel this batch job?')) return;
  try { await apiFetch('/v1/batches/' + id + '/cancel', { method: 'POST' }); toast('Batch cancelled', 'success'); loadBatches(); }
  catch (e) { toast('Failed to cancel batch: ' + e.message, 'error'); }
}

async function viewResults(id) {
  try {
    const resp = await apiFetch('/v1/batches/' + id + '/results');
    document.getElementById('results-modal-desc').textContent = 'Results for batch ' + id;
    document.getElementById('results-content').textContent = JSON.stringify(resp.data || resp || [], null, 2);
    document.getElementById('results-modal').classList.remove('hidden');
  } catch (e) { toast('Failed to load results: ' + e.message, 'error'); }
}

// ===================== ACCESS CONTROL =====================
function loadAccess() {
  loadTeams();
  loadUsers();
  loadOrgs();
}

function switchAccessTab(tab) {
  document.querySelectorAll('#access-tabs .tab').forEach(t => t.classList.remove('active'));
  document.querySelector('#access-tabs .tab[data-tab="' + tab + '"]').classList.add('active');
  ['teams', 'users', 'orgs'].forEach(t => {
    document.getElementById('access-tab-' + t).classList.toggle('hidden', t !== tab);
  });
}

// ===================== TEAMS =====================
async function loadTeams() {
  const tbody = document.getElementById('teams-tbody');
  try {
    const resp = await apiFetch('/teams');
    const teams = resp.data || resp || [];
    if (!Array.isArray(teams) || teams.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/></svg></div><h4>No teams</h4><p>Create a team to organize keys and track budgets.</p></div></td></tr>';
      return;
    }
    tbody.innerHTML = teams.map(t => `<tr>
      <td class="text-mono">${t.id}</td>
      <td style="font-weight:500;color:var(--text-primary)">${esc(t.name)}</td>
      <td>${esc(t.org_id || '—')}</td>
      <td>${t.max_budget ? '$' + parseFloat(t.max_budget).toFixed(2) : '&#8734;'}</td>
      <td class="text-sm text-muted">${t.created_at ? new Date(t.created_at).toLocaleDateString() : '—'}</td>
      <td style="text-align:right"><button class="btn btn-danger btn-sm" onclick="deleteTeam(${t.id})">Delete</button></td>
    </tr>`).join('');
  } catch (e) { toast('Failed to load teams: ' + e.message, 'error'); }
}

async function showCreateTeamModal() {
  document.getElementById('ct-name').value = '';
  document.getElementById('ct-budget').value = '';
  document.getElementById('ct-org').value = '';
  await populateOrgDropdowns();
  document.getElementById('create-team-modal').classList.remove('hidden');
  setTimeout(() => document.getElementById('ct-name').focus(), 100);
}

async function createTeam() {
  const name = document.getElementById('ct-name').value.trim();
  if (!name) { toast('Team name is required', 'error'); return; }
  const orgId = document.getElementById('ct-org').value;
  if (!orgId) { toast('Organization is required', 'error'); return; }
  try {
    await apiFetch('/teams', { method: 'POST', body: JSON.stringify({ name, org_id: orgId, max_budget: parseFloat(document.getElementById('ct-budget').value) || 0 }) });
    document.getElementById('create-team-modal').classList.add('hidden');
    toast('Team created', 'success'); loadTeams();
  } catch (e) { toast('Failed to create team: ' + e.message, 'error'); }
}

async function deleteTeam(id) {
  if (!confirm('Delete this team?')) return;
  try { await apiFetch('/teams/' + id, { method: 'DELETE' }); toast('Team deleted', 'success'); loadTeams(); }
  catch (e) { toast('Failed to delete team: ' + e.message, 'error'); }
}

// ===================== USERS =====================
async function loadUsers() {
  const tbody = document.getElementById('users-tbody');
  try {
    const resp = await apiFetch('/users');
    const users = resp.data || resp || [];
    if (!Array.isArray(users) || users.length === 0) {
      tbody.innerHTML = '<tr><td colspan="8"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg></div><h4>No users</h4><p>Create a user to track individual budgets and access.</p></div></td></tr>';
      return;
    }
    tbody.innerHTML = users.map(u => {
      const roleCls = u.role === 'admin' ? 'badge-active' : 'badge-processing';
      return `<tr>
      <td class="text-mono">${u.id}</td>
      <td style="font-weight:500;color:var(--text-primary)">${esc(u.user_id)}</td>
      <td>${esc(u.email || '—')}</td>
      <td><span class="badge ${roleCls}">${esc(u.role || 'member')}</span></td>
      <td>${esc(u.team_id || '—')}</td>
      <td>${esc(u.org_id || '—')}</td>
      <td>${u.max_budget ? '$' + parseFloat(u.max_budget).toFixed(2) : '&#8734;'}</td>
      <td style="text-align:right"><button class="btn btn-danger btn-sm" onclick="deleteUser(${u.id})">Delete</button></td>
    </tr>`;
    }).join('');
  } catch (e) { toast('Failed to load users: ' + e.message, 'error'); }
}

async function showCreateUserModal() {
  ['cu-userid','cu-email','cu-budget'].forEach(id => document.getElementById(id).value = '');
  document.getElementById('cu-org').value = '';
  document.getElementById('cu-team').value = '';
  await populateDropdowns();
  document.getElementById('create-user-modal').classList.remove('hidden');
  setTimeout(() => document.getElementById('cu-userid').focus(), 100);
}

async function createUser() {
  const userId = document.getElementById('cu-userid').value.trim();
  if (!userId) { toast('User ID is required', 'error'); return; }
  try {
    const orgId = document.getElementById('cu-org').value;
    if (!orgId) { toast('Organization is required', 'error'); return; }
    await apiFetch('/users', { method: 'POST', body: JSON.stringify({ user_id: userId, email: document.getElementById('cu-email').value || undefined, team_id: document.getElementById('cu-team').value || undefined, org_id: orgId, role: document.getElementById('cu-role').value || 'member', max_budget: parseFloat(document.getElementById('cu-budget').value) || 0 }) });
    document.getElementById('create-user-modal').classList.add('hidden');
    toast('User created', 'success'); loadUsers();
  } catch (e) { toast('Failed to create user: ' + e.message, 'error'); }
}

async function deleteUser(id) {
  if (!confirm('Delete this user?')) return;
  try { await apiFetch('/users/' + id, { method: 'DELETE' }); toast('User deleted', 'success'); loadUsers(); }
  catch (e) { toast('Failed to delete user: ' + e.message, 'error'); }
}

// ===================== ORGANIZATIONS =====================
async function loadOrgs() {
  const tbody = document.getElementById('orgs-tbody');
  try {
    const resp = await apiFetch('/organizations');
    const orgs = resp.data || resp || [];
    if (!Array.isArray(orgs) || orgs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg></div><h4>No organizations</h4><p>Create an organization for multi-tenant isolation.</p></div></td></tr>';
      return;
    }
    tbody.innerHTML = orgs.map(o => `<tr>
      <td class="text-mono">${o.id}</td>
      <td style="font-weight:500;color:var(--text-primary)">${esc(o.name)}</td>
      <td>${esc(o.admin_email || '—')}</td>
      <td>${o.max_budget ? '$' + parseFloat(o.max_budget).toFixed(2) : '&#8734;'}</td>
      <td class="text-sm text-muted">${o.created_at ? new Date(o.created_at).toLocaleDateString() : '—'}</td>
      <td style="text-align:right"><button class="btn btn-danger btn-sm" onclick="deleteOrg(${o.id})">Delete</button></td>
    </tr>`).join('');
  } catch (e) { toast('Failed to load organizations: ' + e.message, 'error'); }
}

function showCreateOrgModal() {
  ['co-name','co-admin-email','co-budget'].forEach(id => document.getElementById(id).value = '');
  document.getElementById('create-org-modal').classList.remove('hidden');
  setTimeout(() => document.getElementById('co-name').focus(), 100);
}

async function createOrg() {
  const name = document.getElementById('co-name').value.trim();
  if (!name) { toast('Organization name is required', 'error'); return; }
  const adminEmail = document.getElementById('co-admin-email').value.trim();
  try {
    await apiFetch('/organizations', { method: 'POST', body: JSON.stringify({ name, admin_email: adminEmail || undefined, max_budget: parseFloat(document.getElementById('co-budget').value) || 0 }) });
    document.getElementById('create-org-modal').classList.add('hidden');
    toast('Organization created', 'success'); loadOrgs();
  } catch (e) { toast('Failed to create organization: ' + e.message, 'error'); }
}

async function deleteOrg(id) {
  if (!confirm('Delete this organization?')) return;
  try { await apiFetch('/organizations/' + id, { method: 'DELETE' }); toast('Organization deleted', 'success'); loadOrgs(); }
  catch (e) { toast('Failed to delete organization: ' + e.message, 'error'); }
}

// ===================== PLAYGROUND =====================
async function loadPlayground() {
  const modelSelect = document.getElementById('pg-model');
  // Populate model dropdown from allModels (or fetch if empty)
  if (!allModels || allModels.length === 0) {
    try {
      const resp = await apiFetch('/v1/models');
      const data = resp.data || resp || [];
      allModels = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
    } catch { /* silent */ }
  }
  // Only show chat-capable models in the playground (filter out embedding models)
  const chatModels = allModels.filter(m => !m.type || m.type === 'chat');
  if (chatModels.length > 0) {
    modelSelect.innerHTML = chatModels.map(m => `<option value="${esc(m.id)}">${esc(m.id)}</option>`).join('');
  } else {
    modelSelect.innerHTML = '<option value="">No chat models available</option>';
  }
  setTimeout(() => document.getElementById('pg-message').focus(), 100);
}

async function sendPlaygroundRequest() {
  const model = document.getElementById('pg-model').value;
  const systemPrompt = document.getElementById('pg-system').value.trim();
  const userMessage = document.getElementById('pg-message').value.trim();
  const temperature = parseFloat(document.getElementById('pg-temperature').value);
  const maxTokens = parseInt(document.getElementById('pg-max-tokens').value) || 1000;

  if (!model) { toast('Please select a model', 'error'); return; }
  if (!userMessage) { toast('Please enter a message', 'error'); return; }

  const messages = [];
  if (systemPrompt) messages.push({ role: 'system', content: systemPrompt });
  messages.push({ role: 'user', content: userMessage });

  const request = { model, messages, temperature, max_tokens: maxTokens };

  // Add web search options if enabled
  const wsCheckbox = document.getElementById('pg-websearch');
  if (wsCheckbox && wsCheckbox.checked) {
    const contextSize = document.getElementById('pg-search-context')?.value || 'medium';
    request.web_search_options = { search_context_size: contextSize };
  }

  // Update UI to loading state
  const sendBtn = document.getElementById('pg-send');
  const responseEl = document.getElementById('pg-response');
  const metaEl = document.getElementById('pg-meta');
  sendBtn.disabled = true;
  sendBtn.innerHTML = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg> Sending...';
  responseEl.innerHTML = '<div style="padding:24px;color:var(--text-muted);text-align:center"><svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" style="animation:spin 1s linear infinite;display:inline-block;vertical-align:middle;margin-right:8px"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg> Waiting for response...</div>';
  metaEl.classList.add('hidden');

  // Generate curl command
  const curlCmd = generatePlaygroundCurl(request);
  document.getElementById('pg-curl-content').textContent = curlCmd;

  const startTime = performance.now();

  try {
    const resp = await apiFetch('/v1/chat/completions', { method: 'POST', body: JSON.stringify(request) });
    const latencyMs = Math.round(performance.now() - startTime);
    const data = resp.data || resp;

    // Extract response content
    const choices = data.choices || [];
    const content = choices.length > 0 ? (choices[0].message?.content || choices[0].text || '') : 'No response content';
    const usage = data.usage || {};
    const costVal = data.cost || 0;
    const cached = data.cached || false;
    const respModel = data.model || model;

    // Display response text
    let responseHTML = '<div class="playground-response-text">' + esc(content) + '</div>';

    // Display web search citations if present
    const annotations = data.annotations || [];
    if (annotations.length > 0) {
      const citations = annotations
        .filter(a => a.type === 'url_citation' && a.url_citation)
        .map(a => `<a href="${esc(a.url_citation.url)}" target="_blank" rel="noopener" style="color:var(--accent);text-decoration:none">${esc(a.url_citation.title || a.url_citation.url)}</a>`)
        .join(', ');
      if (citations) {
        responseHTML += '<div style="margin-top:12px;padding-top:12px;border-top:1px solid var(--border);font-size:12px;color:var(--text-muted)"><strong>Sources:</strong> ' + citations + '</div>';
      }
    }

    responseEl.innerHTML = responseHTML;

    // Display metadata
    document.getElementById('pg-meta-model').textContent = respModel;
    document.getElementById('pg-meta-latency').textContent = latencyMs + 'ms';
    document.getElementById('pg-meta-prompt-tokens').textContent = (usage.prompt_tokens || 0).toLocaleString();
    document.getElementById('pg-meta-completion-tokens').textContent = (usage.completion_tokens || 0).toLocaleString();
    document.getElementById('pg-meta-total-tokens').textContent = (usage.total_tokens || 0).toLocaleString();
    document.getElementById('pg-meta-cost').textContent = '$' + (costVal || 0).toFixed(6);
    document.getElementById('pg-meta-cached').innerHTML = cached
      ? '<span class="badge badge-active">Yes</span>'
      : '<span class="badge badge-cancelled">No</span>';
    metaEl.classList.remove('hidden');

  } catch (e) {
    const latencyMs = Math.round(performance.now() - startTime);
    responseEl.innerHTML = '<div style="padding:16px;color:var(--danger)"><strong>Error:</strong> ' + esc(e.message) + '</div>';
    toast('Request failed: ' + e.message, 'error');
  } finally {
    sendBtn.disabled = false;
    sendBtn.innerHTML = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg> Send Request';
  }
}

function generatePlaygroundCurl(request) {
  const baseUrl = window.location.origin;
  const body = JSON.stringify(request, null, 2);
  return `curl -X POST ${baseUrl}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '${body}'`;
}

function switchPlaygroundTab(tab) {
  document.querySelectorAll('.playground-tab').forEach(el => el.classList.remove('active'));
  document.querySelector('.playground-tab[data-tab="' + tab + '"]').classList.add('active');
  document.getElementById('pg-tab-response').classList.toggle('hidden', tab !== 'response');
  document.getElementById('pg-tab-curl').classList.toggle('hidden', tab !== 'curl');
}

function copyPlaygroundCurl() {
  const curlText = document.getElementById('pg-curl-content').textContent;
  navigator.clipboard.writeText(curlText)
    .then(() => toast('Curl command copied to clipboard', 'success'))
    .catch(() => {
      const ta = document.createElement('textarea');
      ta.value = curlText;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      toast('Curl command copied to clipboard', 'success');
    });
}

// ===================== DOCS =====================
function loadDocs() {
  // Track active section on scroll
  const content = document.querySelector('.docs-content');
  if (!content) return;
  const sections = content.querySelectorAll('.docs-section[id]');
  if (!sections.length) return;
  const main = document.querySelector('.main');
  if (!main) return;

  main.addEventListener('scroll', function docsScroll() {
    if (document.getElementById('page-docs').classList.contains('hidden')) return;
    let current = '';
    sections.forEach(sec => {
      const top = sec.getBoundingClientRect().top;
      if (top < 200) current = sec.id;
    });
    if (current) {
      document.querySelectorAll('.docs-nav-item').forEach(a => {
        a.classList.toggle('active', a.dataset.doc === current);
      });
    }
  });
}

function docNav(sectionId) {
  const el = document.getElementById(sectionId);
  if (!el) return;
  el.scrollIntoView({ behavior: 'smooth', block: 'start' });
  document.querySelectorAll('.docs-nav-item').forEach(a => {
    a.classList.toggle('active', a.dataset.doc === sectionId);
  });
}

// ===================== GUARDRAILS =====================
async function loadGuardrails() {
  const tbody = document.getElementById('guardrails-tbody');
  try {
    const resp = await apiFetch('/guardrails');
    const configs = resp.data || resp || [];
    if (!Array.isArray(configs) || configs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg></div><h4>No guardrail configs in DB</h4><p>Env defaults are active. Add a DB config to override them.</p></div></td></tr>';
      return;
    }
    tbody.innerHTML = configs.map(c => {
      const scope = c.key_hash ? `<code class="text-sm">${esc(c.key_hash.substring(0, 12))}...</code>` : '<span class="badge badge-active">Global</span>';
      const piiCls = c.pii_action === 'block' ? 'badge-failed' : c.pii_action === 'redact' ? 'badge-pending' : c.pii_action === 'log' ? 'badge-processing' : 'badge-cancelled';
      return `<tr>
        <td>${scope}</td>
        <td><span class="badge ${piiCls}">${esc(c.pii_action || 'none')}</span></td>
        <td>${c.max_input_tokens || '&#8734;'}</td>
        <td>${c.max_output_tokens || '&#8734;'}</td>
        <td class="text-sm">${esc(c.blocked_keywords || '—')}</td>
        <td><span class="badge ${c.enabled ? 'badge-active' : 'badge-inactive'}">${c.enabled ? 'Yes' : 'No'}</span></td>
        <td style="text-align:right;white-space:nowrap">
          <button class="btn btn-secondary btn-sm" onclick="editGuardrail(${c.id}, '${esc(c.key_hash || '')}', '${esc(c.pii_action || 'none')}', ${c.max_input_tokens || 0}, ${c.max_output_tokens || 0}, '${esc(c.blocked_keywords || '')}', ${c.enabled})">Edit</button>
          <button class="btn btn-danger btn-sm" onclick="deleteGuardrail(${c.id})">Delete</button>
        </td>
      </tr>`;
    }).join('');
  } catch (e) { toast('Failed to load guardrails: ' + e.message, 'error'); }
}

function showGuardrailModal(keyHash, pii, maxIn, maxOut, keywords, enabled) {
  document.getElementById('gr-keyhash').value = keyHash || '';
  document.getElementById('gr-pii').value = pii || 'none';
  document.getElementById('gr-max-in').value = maxIn || '';
  document.getElementById('gr-max-out').value = maxOut || '';
  document.getElementById('gr-keywords').value = keywords || '';
  document.getElementById('gr-enabled').value = enabled !== false ? 'true' : 'false';
  document.getElementById('gr-modal-title').textContent = keyHash ? 'Edit Guardrail Config' : 'Add Guardrail Config';
  document.getElementById('guardrail-modal').classList.remove('hidden');
}

function editGuardrail(id, keyHash, pii, maxIn, maxOut, keywords, enabled) {
  showGuardrailModal(keyHash, pii, maxIn, maxOut, keywords, enabled);
}

async function saveGuardrail() {
  const body = {
    key_hash: document.getElementById('gr-keyhash').value.trim() || undefined,
    pii_action: document.getElementById('gr-pii').value,
    max_input_tokens: parseInt(document.getElementById('gr-max-in').value) || 0,
    max_output_tokens: parseInt(document.getElementById('gr-max-out').value) || 0,
    blocked_keywords: document.getElementById('gr-keywords').value.trim(),
    enabled: document.getElementById('gr-enabled').value === 'true',
  };
  try {
    await apiFetch('/guardrails', { method: 'POST', body: JSON.stringify(body) });
    document.getElementById('guardrail-modal').classList.add('hidden');
    toast('Guardrail config saved', 'success');
    loadGuardrails();
  } catch (e) { toast('Failed to save guardrail: ' + e.message, 'error'); }
}

async function deleteGuardrail(id) {
  if (!confirm('Delete this guardrail config?')) return;
  try { await apiFetch('/guardrails/' + id, { method: 'DELETE' }); toast('Guardrail config deleted', 'success'); loadGuardrails(); }
  catch (e) { toast('Failed to delete: ' + e.message, 'error'); }
}

// ===================== SETTINGS =====================
async function loadSettings() {
  const container = document.getElementById('settings-content');
  try {
    const resp = await apiFetch('/settings');
    const s = resp.data || resp;

    const providerRows = Object.entries(s.providers || {}).map(([name, cfg]) => {
      const status = cfg.configured ? '<span class="badge badge-active">Configured</span>' : '<span class="badge badge-inactive">Not Set</span>';
      const baseUrl = cfg.base_url ? `<div class="text-sm text-muted">${esc(cfg.base_url)}</div>` : '';
      const timeout = cfg.timeout_ms && cfg.timeout_ms !== '0' ? cfg.timeout_ms + 'ms' : 'Default';
      return `<tr><td style="font-weight:500;color:var(--text-primary)">${esc(name)}</td><td>${status}</td><td>${timeout}</td><td>${baseUrl || '—'}</td></tr>`;
    }).join('');

    const routingItems = [
      ['Strategy', s.routing?.strategy || 'simple'],
      ['Default Provider', s.routing?.default_provider || 'openai'],
      ['Retry Max', s.routing?.retry_max || '3'],
      ['Retry Backoff', (s.routing?.retry_backoff_ms || '500') + 'ms'],
      ['Cooldown Threshold', s.routing?.cooldown_threshold || '5'],
      ['Cooldown Period', (s.routing?.cooldown_period_sec || '60') + 's'],
      ['Circuit Breaker Threshold', s.routing?.cb_threshold || '5'],
      ['Circuit Breaker Interval', (s.routing?.cb_interval_sec || '30') + 's'],
      ['Fallback Chain', s.routing?.fallback_chain || '—'],
      ['Request Queue', s.routing?.queue_enabled === 'true' ? 'Enabled (' + s.routing?.queue_size + ')' : 'Disabled'],
    ];

    const grItems = [
      ['Enabled (env)', s.guardrails?.enabled || 'false'],
      ['PII Action (env)', s.guardrails?.pii_action || 'none'],
      ['Blocked Keywords (env)', s.guardrails?.blocked_keywords || '—'],
      ['Max Input Tokens (env)', s.guardrails?.max_input_tokens || '0 (unlimited)'],
      ['Max Output Tokens (env)', s.guardrails?.max_output_tokens || '0 (unlimited)'],
    ];

    container.innerHTML = `
      <div class="card"><div class="card-header"><h3>Providers</h3></div><div class="card-body no-pad">
        <table><thead><tr><th>Provider</th><th>Status</th><th>Timeout</th><th>Base URL</th></tr></thead>
        <tbody>${providerRows}</tbody></table>
      </div></div>
      <div class="settings-grid">
        <div class="card"><div class="card-header"><h3>Routing & Reliability</h3></div><div class="card-body no-pad">
          ${routingItems.map(([k, v]) => `<div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">${esc(k)}</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${esc(String(v))}</div></div>`).join('')}
        </div></div>
        <div class="card"><div class="card-header"><h3>Guardrail Defaults (env)</h3></div><div class="card-body no-pad">
          ${grItems.map(([k, v]) => `<div class="activity-item"><div class="activity-dot ${v === 'true' || v === 'block' || v === 'redact' ? 'success' : 'accent'}"></div><div class="activity-text">${esc(k)}</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${esc(String(v))}</div></div>`).join('')}
        </div></div>
      </div>
      <div class="settings-grid">
        <div class="card"><div class="card-header"><h3>Web Search</h3></div><div class="card-body no-pad">
          <div class="activity-item"><div class="activity-dot ${s.websearch?.enabled === 'true' ? 'success' : 'accent'}"></div><div class="activity-text">Enabled</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${s.websearch?.enabled || 'false'}</div></div>
          <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Provider</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${esc(String(s.websearch?.provider || 'searxng'))}</div></div>
          <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Max Results</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${s.websearch?.max_results || 5}</div></div>
          <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Cache TTL</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${s.websearch?.cache_ttl || 300}s</div></div>
          <div class="activity-item"><div class="activity-dot ${s.websearch?.api_key_set ? 'success' : 'accent'}"></div><div class="activity-text">API Key</div><div class="activity-time">${s.websearch?.api_key_set ? '<span class="badge badge-active">Set</span>' : '<span class="badge badge-inactive">Not Set</span>'}</div></div>
          <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Query Model</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${esc(String(s.websearch?.query_model || 'auto'))}</div></div>
        </div></div>
        <div class="card"><div class="card-header"><h3>Cache</h3></div><div class="card-body no-pad">
          <div class="activity-item"><div class="activity-dot success"></div><div class="activity-text">TTL</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${s.cache?.ttl_seconds || 300}s</div></div>
        </div></div>
        <div class="card"><div class="card-header"><h3>Batch Processing</h3></div><div class="card-body no-pad">
          <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Workers</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${s.batch?.workers || 5}</div></div>
          <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Task Timeout</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${s.batch?.task_timeout_sec || 120}s</div></div>
        </div></div>
      </div>
      <div class="settings-grid">
        <div class="card"><div class="card-header"><h3>Authentication</h3></div><div class="card-body no-pad">
          <div class="activity-item"><div class="activity-dot ${s.auth?.master_key_configured ? 'success' : 'danger'}"></div><div class="activity-text">Master Key</div><div class="activity-time">${s.auth?.master_key_configured ? '<span class="badge badge-active">Set</span>' : '<span class="badge badge-inactive">Not Set</span>'}</div></div>
          <div class="activity-item"><div class="activity-dot accent"></div><div class="activity-text">Gateway Keys</div><div class="activity-time" style="font-weight:500;color:var(--text-primary)">${s.auth?.key_count || 0} configured</div></div>
        </div></div>
      </div>
      <div class="card" style="margin-top:16px"><div class="card-body" style="padding:14px 20px;font-size:12px;color:var(--text-dim);line-height:1.6">
        These values are loaded from environment variables / <code>.env</code> file at startup. To change provider API keys or routing strategy, update the config and restart the gateway. Guardrail overrides can be managed from the <a style="color:var(--accent);cursor:pointer" onclick="navigate('guardrails')">Guardrails</a> page without restart.
      </div></div>
    `;
  } catch (e) { container.innerHTML = '<div class="card"><div class="card-body"><p class="text-muted">Failed to load settings.</p></div></div>'; toast('Failed to load settings: ' + e.message, 'error'); }
  loadProviderConfig();
}

// ===================== PROVIDER CONFIG =====================
function toggleKeyVisibility(inputId) {
  const input = document.getElementById(inputId);
  const btn = input.nextElementSibling;
  if (input.type === 'password') {
    input.type = 'text';
    btn.textContent = 'Hide';
  } else {
    input.type = 'password';
    btn.textContent = 'Show';
  }
}

async function loadProviderConfig() {
  try {
    const resp = await apiFetch('/settings');
    const data = resp.data || resp;
    if (data?.providers) {
      for (const [name, cfg] of Object.entries(data.providers)) {
        const statusEl = document.getElementById('cfg-' + name + '-status');
        if (statusEl) {
          statusEl.textContent = cfg.configured ? '\u2713 Configured' : 'Not configured';
          statusEl.className = 'settings-provider-status ' + (cfg.configured ? 'configured' : 'not-configured');
        }
      }
    }
  } catch (e) { /* ignore */ }
}

async function saveProviderConfig() {
  const config = {};
  const fields = [
    { id: 'cfg-openai-key', key: 'openai_api_key' },
    { id: 'cfg-anthropic-key', key: 'anthropic_api_key' },
    { id: 'cfg-groq-key', key: 'groq_api_key' },
    { id: 'cfg-deepseek-key', key: 'deepseek_api_key' },
    { id: 'cfg-gemini-key', key: 'gemini_api_key' },
    { id: 'cfg-ollama-url', key: 'ollama_base_url' },
  ];
  for (const f of fields) {
    const val = document.getElementById(f.id).value.trim();
    if (val) config[f.key] = val;
  }
  if (Object.keys(config).length === 0) {
    toast('No changes to save', 'info');
    return;
  }
  try {
    await apiFetch('/settings/providers', { method: 'PUT', body: JSON.stringify(config) });
    toast('Provider config saved. Restart gateway to apply.', 'success');
    document.getElementById('provider-config-status').textContent = 'Saved \u2014 restart to apply';
    loadProviderConfig();
  } catch (e) {
    toast('Failed to save: ' + e.message, 'error');
  }
}

// ===================== AUDIT LOG =====================
async function loadAudit() {
  const entity = document.getElementById('audit-entity-filter').value;
  const action = document.getElementById('audit-action-filter').value;
  let url = '/audit/log?limit=100';
  if (entity) url += '&entity_type=' + entity;
  if (action) url += '&action=' + action;
  const tbody = document.getElementById('audit-tbody');
  try {
    const resp = await apiFetch(url);
    const logs = resp.data || resp || [];
    if (!Array.isArray(logs) || logs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7"><div class="empty-state"><div class="empty-state-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></div><h4>No audit entries</h4><p>Administrative actions will be logged here.</p></div></td></tr>';
      return;
    }
    tbody.innerHTML = logs.map(l => {
      const cls = l.action === 'create' ? 'badge-active' : l.action === 'delete' ? 'badge-failed' : 'badge-processing';
      return `<tr>
        <td class="text-mono text-sm">${l.id}</td>
        <td><span class="badge ${cls}">${esc(l.action)}</span></td>
        <td>${esc(l.entity_type)}</td>
        <td class="text-mono">${esc(l.entity_id || '—')}</td>
        <td>${esc(l.actor_id || 'admin')}</td>
        <td class="text-sm text-muted" style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${esc(l.details || '')}">${esc(l.details || '—')}</td>
        <td class="text-sm text-muted">${l.created_at ? new Date(l.created_at).toLocaleString() : '—'}</td>
      </tr>`;
    }).join('');
  } catch (e) { toast('Failed to load audit log: ' + e.message, 'error'); }
}

// ===================== DROPDOWN POPULATION =====================
async function populateOrgDropdowns() {
  try {
    const resp = await apiFetch('/organizations');
    const orgs = resp.data || resp || [];
    const items = Array.isArray(orgs) ? orgs : [];
    document.querySelectorAll('[data-dropdown="orgs"]').forEach(sel => {
      const current = sel.value;
      sel.innerHTML = '<option value="">— None —</option>' + items.map(o => `<option value="${o.id}">${esc(o.name)} (#${o.id})</option>`).join('');
      sel.value = current;
    });
  } catch { /* silent */ }
}

async function populateTeamDropdowns() {
  try {
    const resp = await apiFetch('/teams');
    const teams = resp.data || resp || [];
    const items = Array.isArray(teams) ? teams : [];
    document.querySelectorAll('[data-dropdown="teams"]').forEach(sel => {
      const current = sel.value;
      sel.innerHTML = '<option value="">— None —</option>' + items.map(t => `<option value="${t.id}">${esc(t.name)} (#${t.id})</option>`).join('');
      sel.value = current;
    });
  } catch { /* silent */ }
}

async function populateDropdowns() {
  await Promise.all([populateOrgDropdowns(), populateTeamDropdowns()]);
}

// ===================== HELPERS =====================
function esc(s) {
  if (s == null) return '';
  const div = document.createElement('div');
  div.textContent = String(s);
  return div.innerHTML;
}

// ===================== INIT =====================
document.addEventListener('DOMContentLoaded', () => {
  // Login
  document.getElementById('login-btn').addEventListener('click', doLogin);
  document.getElementById('login-key').addEventListener('keypress', e => { if (e.key === 'Enter') doLogin(); });

  // Navigation — bind all page nav items (querySelectorAll to handle duplicate IDs across admin/user sidebars)
  allPageIds.forEach(p => {
    document.querySelectorAll('[id="nav-' + p + '"]').forEach(el => {
      el.addEventListener('click', () => navigate(p));
    });
  });

  // Keys
  document.getElementById('btn-generate-key').addEventListener('click', showGenerateKeyModal);
  document.getElementById('gk-cancel').addEventListener('click', hideGenerateKeyModal);
  document.getElementById('gk-submit').addEventListener('click', generateKey);
  document.getElementById('key-reveal-close').addEventListener('click', () => document.getElementById('key-reveal-modal').classList.add('hidden'));

  // Teams
  document.getElementById('btn-create-team').addEventListener('click', showCreateTeamModal);
  document.getElementById('ct-cancel').addEventListener('click', () => document.getElementById('create-team-modal').classList.add('hidden'));
  document.getElementById('ct-submit').addEventListener('click', createTeam);

  // Users
  document.getElementById('btn-create-user').addEventListener('click', showCreateUserModal);
  document.getElementById('cu-cancel').addEventListener('click', () => document.getElementById('create-user-modal').classList.add('hidden'));
  document.getElementById('cu-submit').addEventListener('click', createUser);

  // Orgs
  document.getElementById('btn-create-org').addEventListener('click', showCreateOrgModal);
  document.getElementById('co-cancel').addEventListener('click', () => document.getElementById('create-org-modal').classList.add('hidden'));
  document.getElementById('co-submit').addEventListener('click', createOrg);

  // Guardrails
  document.getElementById('btn-add-guardrail').addEventListener('click', () => showGuardrailModal('', 'none', 0, 0, '', true));
  document.getElementById('gr-cancel').addEventListener('click', () => document.getElementById('guardrail-modal').classList.add('hidden'));
  document.getElementById('gr-submit').addEventListener('click', saveGuardrail);

  // Batch results
  document.getElementById('results-close').addEventListener('click', () => document.getElementById('results-modal').classList.add('hidden'));

  // Logout
  document.getElementById('btn-logout').addEventListener('click', doLogout);

  // Dashboard refresh
  document.getElementById('btn-refresh-dash').addEventListener('click', () => isAdmin() ? loadDashboard() : loadUserDashboard());

  // Spend group-by
  document.getElementById('spend-group-by').addEventListener('change', loadSpend);
  document.getElementById('myspend-group-by').addEventListener('change', loadMySpend);

  // Audit filters
  document.getElementById('audit-entity-filter').addEventListener('change', loadAudit);
  document.getElementById('audit-action-filter').addEventListener('change', loadAudit);

  // Playground
  document.getElementById('pg-send').addEventListener('click', sendPlaygroundRequest);
  document.getElementById('pg-temperature').addEventListener('input', e => {
    document.getElementById('pg-temp-value').textContent = e.target.value;
  });
  document.getElementById('pg-message').addEventListener('keydown', e => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); sendPlaygroundRequest(); }
  });

  // Web search toggle in playground
  const wsCheckbox = document.getElementById('pg-websearch');
  if (wsCheckbox) {
    wsCheckbox.addEventListener('change', () => {
      const optsEl = document.getElementById('pg-websearch-opts');
      if (optsEl) optsEl.style.display = wsCheckbox.checked ? '' : 'none';
    });
  }

  // Model search
  document.getElementById('models-search').addEventListener('input', e => {
    const q = e.target.value.toLowerCase();
    if (!q) { renderModels(allModels); return; }
    renderModels(allModels.filter(m => (m.id || '').toLowerCase().includes(q)));
  });

  // Close modals
  document.querySelectorAll('.modal-overlay').forEach(overlay => {
    overlay.addEventListener('click', e => { if (e.target === overlay) overlay.classList.add('hidden'); });
  });
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') document.querySelectorAll('.modal-overlay:not(.hidden)').forEach(m => m.classList.add('hidden'));
  });

  // Auto-login
  if (masterKey) {
    if (userRole) { showApp(); }
    else {
      apiFetch('/auth/check').then(resp => {
        const data = resp.data || resp;
        userRole = data.role || 'user';
        sessionStorage.setItem('gw_role', userRole);
        showApp();
      }).catch(() => showLogin());
    }
  } else { showLogin(); }
});
