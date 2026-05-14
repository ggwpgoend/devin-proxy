package admin

const dashboardHTML = `{{define "dashboard"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Devin Proxy — Admin</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{--bg:#0d1117;--card:#161b22;--border:#30363d;--text:#c9d1d9;--text2:#8b949e;--accent:#58a6ff;--green:#3fb950;--red:#f85149;--orange:#d29922;--purple:#bc8cff}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;background:var(--bg);color:var(--text);line-height:1.5;min-height:100vh}
a{color:var(--accent);text-decoration:none}
.container{max-width:1200px;margin:0 auto;padding:20px}
header{display:flex;align-items:center;justify-content:space-between;padding:16px 0;border-bottom:1px solid var(--border);margin-bottom:24px}
header h1{font-size:20px;font-weight:600;display:flex;align-items:center;gap:8px}
header h1 span{color:var(--accent)}
.badge{display:inline-block;padding:2px 8px;border-radius:12px;font-size:12px;font-weight:500}
.badge-green{background:rgba(63,185,80,.15);color:var(--green)}
.badge-red{background:rgba(248,81,73,.15);color:var(--red)}
.badge-orange{background:rgba(210,153,34,.15);color:var(--orange)}
.badge-purple{background:rgba(188,140,255,.15);color:var(--purple)}

/* Stats Grid */
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:16px;margin-bottom:24px}
.stat-card{background:var(--card);border:1px solid var(--border);border-radius:8px;padding:16px}
.stat-card .label{font-size:12px;color:var(--text2);text-transform:uppercase;letter-spacing:.5px;margin-bottom:4px}
.stat-card .value{font-size:28px;font-weight:700}
.stat-card .value.green{color:var(--green)}
.stat-card .value.orange{color:var(--orange)}
.stat-card .value.red{color:var(--red)}

/* Sections */
.section{background:var(--card);border:1px solid var(--border);border-radius:8px;margin-bottom:24px;overflow:hidden}
.section-header{display:flex;align-items:center;justify-content:space-between;padding:12px 16px;border-bottom:1px solid var(--border);font-weight:600;font-size:14px}

/* Forms */
.form-row{display:flex;gap:8px;padding:12px 16px;border-bottom:1px solid var(--border);flex-wrap:wrap;align-items:end}
.form-group{display:flex;flex-direction:column;gap:4px}
.form-group label{font-size:11px;color:var(--text2);text-transform:uppercase;letter-spacing:.5px}
input[type="text"],input[type="password"],select,textarea{background:var(--bg);border:1px solid var(--border);border-radius:6px;padding:6px 10px;color:var(--text);font-size:13px;outline:none}
input:focus,textarea:focus,select:focus{border-color:var(--accent)}
textarea{resize:vertical;min-height:60px;font-family:monospace}
button,.btn{background:var(--accent);color:#fff;border:none;border-radius:6px;padding:6px 14px;font-size:13px;cursor:pointer;font-weight:500}
button:hover,.btn:hover{opacity:.9}
.btn-danger{background:var(--red)}
.btn-secondary{background:var(--border);color:var(--text)}
.btn-sm{padding:3px 8px;font-size:11px}

/* Table */
table{width:100%;border-collapse:collapse;font-size:13px}
th{text-align:left;padding:8px 12px;color:var(--text2);font-weight:500;font-size:11px;text-transform:uppercase;letter-spacing:.5px;border-bottom:1px solid var(--border)}
td{padding:8px 12px;border-bottom:1px solid var(--border)}
tr:last-child td{border-bottom:none}
tr:hover{background:rgba(88,166,255,.04)}

/* State badges */
.state-active{color:var(--green)}
.state-cooldown{color:var(--orange)}
.state-disabled{color:var(--text2)}
.state-revoked{color:var(--red)}

/* Status badges for HTTP codes */
.http-success{color:var(--green)}
.http-error{color:var(--red)}
.http-warning{color:var(--orange)}

/* Two-column layout */
.grid-2{display:grid;grid-template-columns:1fr 1fr;gap:16px}
@media(max-width:768px){.grid-2{grid-template-columns:1fr}}

/* Tabs */
.tabs{display:flex;gap:0;border-bottom:1px solid var(--border);padding:0 16px}
.tab{padding:8px 16px;cursor:pointer;color:var(--text2);font-size:13px;border-bottom:2px solid transparent;transition:all .15s}
.tab:hover{color:var(--text)}
.tab.active{color:var(--accent);border-bottom-color:var(--accent)}
.tab-content{display:none;padding:16px}
.tab-content.active{display:block}

/* Proxy info */
.proxy-info{background:rgba(88,166,255,.08);border:1px solid rgba(88,166,255,.2);border-radius:8px;padding:12px 16px;margin-bottom:24px;font-size:13px;display:flex;align-items:center;gap:12px}
.proxy-info code{background:var(--bg);padding:2px 8px;border-radius:4px;font-size:12px;color:var(--accent)}
.mono{font-family:'SF Mono',SFMono-Regular,Consolas,monospace}
.actions{display:flex;gap:4px}
</style>
</head>
<body>
<div class="container">

<header>
  <h1><span>⚡</span> Devin Proxy</h1>
  <div style="font-size:13px;color:var(--text2)">Auto-rotating API key proxy for app.devin.ai</div>
</header>

<div class="proxy-info">
  <strong>Proxy endpoint:</strong>
  <code>http://localhost:9090</code>
  <span style="color:var(--text2)">→</span>
  <code>https://api.devin.ai</code>
  <span style="color:var(--text2);margin-left:auto">Keys auto-rotate on 401/402/429</span>
</div>

<!-- Stats -->
<div class="stats">
  <div class="stat-card">
    <div class="label">Total Keys</div>
    <div class="value">{{.Stats.TotalKeys}}</div>
  </div>
  <div class="stat-card">
    <div class="label">Active</div>
    <div class="value green">{{.Stats.ActiveKeys}}</div>
  </div>
  <div class="stat-card">
    <div class="label">Cooldown</div>
    <div class="value orange">{{.Stats.CooldownKeys}}</div>
  </div>
  <div class="stat-card">
    <div class="label">Revoked / Disabled</div>
    <div class="value red">{{.Stats.RevokedKeys}} / {{.Stats.DisabledKeys}}</div>
  </div>
  <div class="stat-card">
    <div class="label">Total Requests</div>
    <div class="value">{{.Stats.TotalReqs}}</div>
  </div>
  <div class="stat-card">
    <div class="label">Success / Fail</div>
    <div class="value"><span class="green">{{.Stats.SuccessReqs}}</span> / <span class="red">{{.Stats.FailReqs}}</span></div>
  </div>
</div>

<!-- Keys Management -->
<div class="section">
  <div class="section-header">
    <span>API Keys</span>
  </div>
  <div class="tabs">
    <div class="tab active" onclick="switchTab(this,'tab-single')">Add Single Key</div>
    <div class="tab" onclick="switchTab(this,'tab-bulk')">Bulk Import</div>
  </div>
  <div id="tab-single" class="tab-content active">
    <form method="POST" action="/keys">
      <div class="form-row">
        <div class="form-group" style="flex:2">
          <label>API Key</label>
          <input type="password" name="api_key" placeholder="pat_..." required>
        </div>
        <div class="form-group" style="flex:1">
          <label>Label (optional)</label>
          <input type="text" name="label" placeholder="my-account-1">
        </div>
        <div class="form-group">
          <label>Plan</label>
          <select name="plan_type">
            <option value="unknown">Unknown</option>
            <option value="trial">Trial</option>
            <option value="free">Free</option>
            <option value="paid">Paid</option>
          </select>
        </div>
        <button type="submit">Add Key</button>
      </div>
    </form>
  </div>
  <div id="tab-bulk" class="tab-content">
    <form method="POST" action="/keys/bulk">
      <div class="form-row" style="flex-direction:column;gap:8px">
        <div class="form-group" style="width:100%">
          <label>Paste keys (one per line or comma-separated)</label>
          <textarea name="keys" rows="5" placeholder="pat_key1&#10;pat_key2&#10;pat_key3"></textarea>
        </div>
        <div style="display:flex;gap:8px;align-items:end">
          <div class="form-group">
            <label>Plan</label>
            <select name="plan_type">
              <option value="unknown">Unknown</option>
              <option value="trial">Trial</option>
              <option value="free">Free</option>
              <option value="paid">Paid</option>
            </select>
          </div>
          <button type="submit">Import All</button>
        </div>
      </div>
    </form>
  </div>

  <table>
    <thead>
      <tr>
        <th>Label</th>
        <th>State</th>
        <th>Plan</th>
        <th>Requests</th>
        <th>Success</th>
        <th>Fails</th>
        <th>Last Error</th>
        <th>Actions</th>
      </tr>
    </thead>
    <tbody>
    {{range .Keys}}
      <tr>
        <td class="mono" style="font-size:12px">{{.Label}}</td>
        <td><span class="{{stateClass .State}}">{{.State}}</span></td>
        <td>{{.PlanType}}</td>
        <td>{{.RequestCount}}</td>
        <td class="green">{{.SuccessCount}}</td>
        <td class="red">{{.FailCount}}</td>
        <td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="{{.LastError}}">{{truncate .LastError 40}}</td>
        <td class="actions">
          {{if eq .State "active"}}
            <form method="POST" action="/keys/{{.ID}}/disable" style="display:inline"><button class="btn-sm btn-secondary">Disable</button></form>
          {{else if eq .State "disabled"}}
            <form method="POST" action="/keys/{{.ID}}/enable" style="display:inline"><button class="btn-sm" style="background:var(--green)">Enable</button></form>
          {{end}}
          <button class="btn-sm btn-danger" onclick="deleteKey('{{.ID}}')">Delete</button>
        </td>
      </tr>
    {{else}}
      <tr><td colspan="8" style="text-align:center;color:var(--text2);padding:24px">No keys added yet. Add your first API key above.</td></tr>
    {{end}}
    </tbody>
  </table>
</div>

<!-- Recent Requests -->
<div class="section">
  <div class="section-header">
    <span>Recent Requests</span>
    <span style="font-size:12px;color:var(--text2)">Last 50 proxied requests</span>
  </div>
  <table>
    <thead>
      <tr>
        <th>Time</th>
        <th>Key</th>
        <th>Method</th>
        <th>Path</th>
        <th>Status</th>
        <th>Latency</th>
        <th>Error</th>
      </tr>
    </thead>
    <tbody>
    {{range .Logs}}
      <tr>
        <td style="font-size:11px;color:var(--text2);white-space:nowrap">{{.CreatedAt.Format "15:04:05"}}</td>
        <td class="mono" style="font-size:11px">{{.KeyLabel}}</td>
        <td>{{.Method}}</td>
        <td class="mono" style="font-size:11px;max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{.Path}}</td>
        <td><span class="{{if and (ge .StatusCode 200) (lt .StatusCode 300)}}http-success{{else if ge .StatusCode 400}}http-error{{else}}http-warning{{end}}">{{.StatusCode}}</span></td>
        <td style="font-size:11px">{{.LatencyMs}}ms</td>
        <td style="font-size:11px;color:var(--red);max-width:150px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{.ErrorMsg}}</td>
      </tr>
    {{else}}
      <tr><td colspan="7" style="text-align:center;color:var(--text2);padding:24px">No requests yet. Point your client to <code>http://localhost:9090</code></td></tr>
    {{end}}
    </tbody>
  </table>
</div>

</div>

<script>
function switchTab(el, id) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
  el.classList.add('active');
  document.getElementById(id).classList.add('active');
}
function deleteKey(id) {
  if (!confirm('Delete this key?')) return;
  fetch('/keys/' + id, {method: 'DELETE'}).then(() => location.reload());
}
// Auto-refresh every 10s
setTimeout(() => location.reload(), 10000);
</script>
</body>
</html>{{end}}`
