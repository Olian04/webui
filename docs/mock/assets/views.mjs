// Every piece of markup, in one place. The build script imports this to write
// the static files; the browser imports the same module to re-render a leaf
// when a query argument is set. One implementation, two callers.

import { DEVICES, ALERTS, MAXRATE, SITES, STATUSES, MAXRATE as _M, eventsFor } from './data.mjs';

export const esc = s => String(s).replace(/[&<>"]/g, c => ({ '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;' }[c]));

const I = {
  info:   '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
  kebab:  '<circle cx="12" cy="5" r="1.4" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="12" cy="19" r="1.4" fill="currentColor" stroke="none"/>',
  refresh:'<path d="M21 12a9 9 0 1 1-2.6-6.4M21 3v6h-6"/>',
  chev:   '<path d="M6 9l6 6 6-6"/>',
  ok:     '<circle cx="12" cy="12" r="9"/><path d="M8.5 12.5l2.5 2.5 4.5-5"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.4-3.4"/>',
  alert:  '<circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/>',
  lock:   '<rect x="4" y="10" width="16" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
  device: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>',
  bell:   '<path d="M12 3a6 6 0 0 0-6 6c0 5-2 6-2 6h16s-2-1-2-6a6 6 0 0 0-6-6zM10.3 20a2 2 0 0 0 3.4 0"/>',
  gear:   '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9M4.6 9a1.7 1.7 0 0 0-.3-1.9M12 3v2M12 19v2M4.2 7.5l1.7 1M18.1 15.5l1.7 1M4.2 16.5l1.7-1M18.1 8.5l1.7-1"/>',
  flag:   '<path d="M5 21V4h13l-2.5 4L18 12H5"/>',
};
const svg = (d, cls = '') => `<svg class="${cls}" viewBox="0 0 24 24">${d}</svg>`;
const GUARD = 'Requires the editor role';

// --------------------------------------------------------------- addressing
// A path argument is a path segment, so it is a real file. A query argument is
// a query string, so it rides on the same file. That is ArgSpec.InPath, made
// literal by the filesystem.
export const PAGES = {
  devices:   { path: '/index.html',     title: 'Devices',   nav: 'devices' },
  device:    { path: null,              title: 'Device',    nav: 'devices' },
  alerts:    { path: '/alert.html',     title: 'Alerts',    nav: 'alerts'  },
  ingest:    { path: '/settings.html',  title: 'Ingest',    nav: 'ingest',    group: 'Settings' },
  retention: { path: '/retention.html', title: 'Retention', nav: 'retention', group: 'Settings' },
};
export const devicePath = id => `/device/${id}.html`;

export function url(path, args = {}) {
  const q = new URLSearchParams();
  // A zero value is absent and stays out of the URL — the same rule the
  // decoder applies on the way back in.
  Object.entries(args).forEach(([k, v]) => {
    if (v === '' || v == null || v === false || v === 0 || v === '0') return;
    q.set(k, v);
  });
  const s = q.toString();
  return path + (s ? '?' + s : '');
}
const here = (path, args, patch) => url(path, { ...args, ...patch });

// ------------------------------------------------------------------ sidebar
export function navHTML(active) {
  const item = (id, label, href, icon, count) =>
    `<a class="nav-item${id === active ? ' active' : ''}" href="${href}">${icon ? svg(icon, 'ni') : ''}<span>${label}</span>${
      count != null ? `<span class="nav-count">${count}</span>` : ''}</a>`;
  const settingsOpen = active === 'ingest' || active === 'retention';
  return `<div class="nav-section">Platform</div>
${item('devices', 'Devices', PAGES.devices.path, I.device, DEVICES.length)}
${item('alerts', 'Alerts', PAGES.alerts.path, I.bell, ALERTS.filter(a => !a.ack).length)}
<div class="nav-section">Configuration</div>
<div class="nav-item${settingsOpen ? ' open' : ''}" data-group="settings">${svg(I.gear, 'ni')}<span>Settings</span>${svg(I.chev, 'chev')}</div>
<div class="nav-sub${settingsOpen ? ' open' : ''}" data-sub="settings">
${item('ingest', 'Ingest', PAGES.ingest.path)}
${item('retention', 'Retention', PAGES.retention.path)}
</div>
<div class="nav-section">States</div>
${item('forbidden', 'Not permitted', '/forbidden.html', I.lock)}
${item('compile', 'Compile error', '/compile-error.html', I.flag)}`;
}

export const crumbsHTML = parts => parts.map(([label, href], i) => {
  const last = i === parts.length - 1;
  const el = last ? `<span class="cur">${esc(label)}</span>`
    : href ? `<a href="${href}">${esc(label)}</a>` : `<a>${esc(label)}</a>`;
  return (i ? '<span class="sep">&rsaquo;</span>' : '') + el;
}).join('');

// ------------------------------------------------------- arguments → toolbar
export const VARS = {
  devices: [
    { name: 'site',   label: 'Site',   kind: 'select', options: SITES,    all: 'All' },
    { name: 'status', label: 'Status', kind: 'select', options: STATUSES, all: 'All' },
    { name: 'q',      label: 'Filter', kind: 'text',   placeholder: 'id, ip or site…' },
  ],
  device: [{ name: 'tab', label: 'Tab', kind: 'select', options: ['overview', 'raw'], all: 'overview' }],
  alerts: [
    { name: 'sev',   label: 'Severity', kind: 'select', options: ['critical', 'warning', 'info'], all: 'All' },
    { name: 'state', label: 'State',    kind: 'select', options: ['open', 'acknowledged'],        all: 'All' },
  ],
  ingest: [], retention: [],
};

export function toolbarHTML(pageId, path, args) {
  const vars = VARS[pageId] || [];
  const pills = vars.map(v => {
    const val = args[v.name] || '';
    const ctl = v.kind === 'select'
      ? `<select data-var="${v.name}">
           <option value="">${esc(v.all)}</option>
           ${v.options.map(o => `<option${val === o ? ' selected' : ''}>${esc(o)}</option>`).join('')}
         </select>`
      : `<form data-varform method="get" action="${path}">
           ${Object.entries(args).filter(([k]) => k !== v.name && k !== 'offset')
             .map(([k, x]) => `<input type="hidden" name="${esc(k)}" value="${esc(x)}">`).join('')}
           <input name="${v.name}" data-var="${v.name}" value="${esc(val)}" placeholder="${esc(v.placeholder || '')}">
         </form>`;
    return `<div class="var">
      <span class="var-label">${esc(v.label)}${svg(I.info)}</span>
      <div class="var-ctl">${ctl}${val ? `<a class="var-x" href="${here(path, args, { [v.name]: '', offset: 0 })}">&times;</a>` : ''}</div>
    </div>`;
  }).join('');
  const dirty = vars.some(v => args[v.name]);
  return (pills || '<span class="noargs">This page declares no arguments.</span>') +
    `<div class="tb-right">
      ${dirty ? `<a class="tb-btn" href="${path}">Clear all</a>` : ''}
      <a class="tb-btn split-l" href="${url(path, args)}">${svg(I.refresh)}Refresh</a>
      <button class="tb-btn split-r" data-act="refresh-menu">${svg(I.chev)}</button>
    </div>`;
}

// ------------------------------------------------------------------- panels
function panel(o) {
  return `<div class="panel" data-leaf="${o.leaf || ''}">
  <div class="panel-head">
    <h2 class="panel-title">${esc(o.title)}</h2>
    ${o.desc ? `<svg class="info" viewBox="0 0 24 24"><title>${esc(o.desc)}</title>${I.info}</svg>` : ''}
    ${o.ok ? `<svg class="panel-status ok" viewBox="0 0 24 24"><title>Loaded</title>${I.ok}</svg>` : ''}
    <div class="panel-menu">${o.actions || ''}<button class="iconbtn" data-act="refresh-leaf">${svg(I.kebab)}</button></div>
  </div>
  ${o.body}
  ${o.foot ? `<div class="panel-foot">${o.foot}</div>` : ''}
</div>`;
}
export const row = (id, label, count, inner) =>
  `<div class="rowhead" data-row="${id}">${svg(I.chev)}${esc(label)}${count != null ? `<span class="cnt">${count}</span>` : ''}</div>
<div class="rowbody" data-rowbody="${id}">${inner}</div>`;

export const note = html => `<div class="note">${html}</div>`;

// ------------------------------------------------------------ cell renderers
const caret = dir => `<svg class="caret" viewBox="0 0 24 24"><path d="${dir === 'asc' ? 'M6 15l6-6 6 6' : 'M6 9l6 6 6-6'}"/></svg>`;
const statusBadge = s => s === 'healthy' ? '<span class="badge badge-ok"><span class="dot"></span>healthy</span>'
  : s === 'degraded' ? '<span class="badge badge-warn"><span class="dot"></span>degraded</span>'
  : '<span class="badge badge-mute"><span class="dot"></span>quiet</span>';
const sevBadge = s => s === 'critical' ? '<span class="badge badge-bad">critical</span>'
  : s === 'warning' ? '<span class="badge badge-warn">warning</span>'
  : '<span class="badge badge-mute">info</span>';
const gauge = v => `<span class="gauge"><span>${v.toFixed(2)}</span>
  <span class="gauge-track"><span class="gauge-fill" style="width:${Math.max(3, Math.round(v / MAXRATE * 100))}%"></span></span></span>`;

export const skeletonRows = (cols, n = 6) => Array.from({ length: n }, () =>
  `<tr class="skel-row">${Array.from({ length: cols }, (_, i) =>
    `<td><div class="skel" style="width:${[70, 55, 40, 35, 50, 60][i % 6]}%"></div></td>`).join('')}</tr>`).join('');

// -------------------------------------------------------------------- leaves
export function devicesPanel(args = {}) {
  const path = PAGES.devices.path;
  const sort = args.sort || 'count', dir = args.dir || 'desc';
  const offset = +(args.offset || 0), size = 10;
  const q = (args.q || '').toLowerCase();

  const rows = DEVICES.filter(d =>
    (!q || d.id.includes(q) || d.ip.includes(q) || d.site.toLowerCase().includes(q)) &&
    (!args.site || d.site === args.site) &&
    (!args.status || d.status === args.status))
    .sort((a, b) => (a[sort] > b[sort] ? 1 : a[sort] < b[sort] ? -1 : 0) * (dir === 'asc' ? 1 : -1));
  const total = rows.length;
  const page = rows.slice(offset, offset + size);

  const th = (key, label, cls = '') => {
    const on = sort === key;
    return `<th class="${cls}${on ? ' sorted' : ''}">
      <a href="${here(path, args, { sort: key, dir: on && dir === 'desc' ? 'asc' : 'desc', offset: 0 })}">${label}${on ? caret(dir) : ''}</a></th>`;
  };

  const body = page.length ? page.map(d => `
    <tr class="clickable" data-href="${devicePath(d.id)}">
      <td class="mono dim"><a href="${devicePath(d.id)}">${d.id}</a></td>
      <td class="mono">${d.ip}</td>
      <td>${statusBadge(d.status)}</td>
      <td class="dim">${esc(d.site)}</td>
      <td class="num">${d.count.toLocaleString('en-US')}</td>
      <td class="num">${gauge(d.rate)}</td>
    </tr>`).join('')
    : `<tr><td colspan="6"><div class="empty">${svg(I.search)}
        <div class="empty-title">No devices match the current arguments</div>
        <div class="empty-desc">Clear them in the toolbar to see all ${DEVICES.length}.</div></div></td></tr>`;

  return panel({
    leaf: 'devices', title: 'Devices', ok: true,
    desc: 'One Table leaf. Sort, page and row links are all URLs; it reloads on its own when an argument it reads changes.',
    body: `<div class="panel-body"><table>
      <thead><tr>${th('id', 'ID')}${th('ip', 'IP')}${th('status', 'Status')}${th('site', 'Site')}${th('count', 'Occurrences', 'num')}${th('rate', 'Rate / s', 'num')}</tr></thead>
      <tbody>${body}</tbody></table></div>`,
    foot: `<div class="pager-info">${total ? `${offset + 1}–${Math.min(offset + size, total)} of ${total}` : 'No results'}</div>
      <div class="pager">
        ${offset > 0 ? `<a class="btn btn-secondary btn-sm" href="${here(path, args, { offset: Math.max(0, offset - size) })}">Previous</a>`
                     : '<span class="btn btn-secondary btn-sm disabled">Previous</span>'}
        ${offset + size < total ? `<a class="btn btn-secondary btn-sm" href="${here(path, args, { offset: offset + size })}">Next</a>`
                                : '<span class="btn btn-secondary btn-sm disabled">Next</span>'}
      </div>`,
  });
}

export function alertsPanel(args = {}) {
  const rows = ALERTS.filter(a => (!args.sev || a.severity === args.sev) &&
    (!args.state || (args.state === 'acknowledged' ? a.ack : !a.ack)));
  return panel({
    leaf: 'alerts', title: 'Alerts', ok: true,
    desc: 'Declares BulkActions, so it gets checkboxes and an action bar. The devices table declares none and gets neither.',
    body: `<form method="post" action="${PAGES.alerts.path}">
      <div class="actionbar">
        <strong data-selcount>0 selected</strong><span class="dim">&middot;</span>
        <button class="btn btn-sm btn-secondary" data-guard="${GUARD}" name="act" value="ack">Acknowledge</button>
        <button class="btn btn-sm btn-danger" data-guard="${GUARD}" name="act" value="dismiss">Dismiss</button>
      </div>
      <div class="panel-body"><table>
        <thead><tr><th class="pick"><input class="checkbox" type="checkbox" data-pickall></th>
          <th style="width:110px">Severity</th><th style="width:150px">Device</th><th>Message</th><th style="width:140px">State</th></tr></thead>
        <tbody>${rows.length ? rows.map(a => `
          <tr><td class="pick"><input class="checkbox" type="checkbox" name="id" value="${a.id}"></td>
            <td>${sevBadge(a.severity)}</td>
            <td class="mono dim"><a href="${devicePath(a.device)}">${a.device}</a></td>
            <td>${esc(a.message)}</td>
            <td>${a.ack ? '<span class="badge badge-mute">acknowledged</span>' : '<span class="badge badge-warn">open</span>'}</td>
          </tr>`).join('') : `<tr><td colspan="5"><div class="empty">${svg(I.ok)}
            <div class="empty-title">Nothing matches</div>
            <div class="empty-desc">Clear the severity or state argument.</div></div></td></tr>`}
        </tbody></table></div></form>`,
  });
}

export function deviceFormPanel(d, fieldErr) {
  const ipErr = fieldErr && fieldErr.field === 'ip' ? fieldErr.message : null;
  return panel({
    leaf: 'form', title: 'Configuration',
    desc: 'A Form leaf. Fields whose accessor declares no Store render read-only.',
    body: `<form data-device-form method="post" action="${devicePath(d.id)}" novalidate>
      <div class="panel-body pad">
        <div class="group"><div class="group-legend">Identity</div>
          <div class="field-row">
            <div class="field"><label class="lbl" for="f-id">ID</label>
              <input class="input mono" id="f-id" value="${esc(d.id)}" readonly>
              <div class="hint">Read-only — the accessor declares no <code>Store</code>.</div></div>
            <div class="field"><label class="lbl" for="f-ip">IP <span class="req">*</span></label>
              <input class="input mono${ipErr ? ' invalid' : ''}" id="f-ip" name="ip" value="${esc(d.ip)}"
                     required minlength="7" maxlength="15" pattern="^\\d{1,3}(\\.\\d{1,3}){3}$">
              ${ipErr ? `<div class="err">${svg(I.alert)}${esc(ipErr)}</div>`
                      : '<div class="hint">IPv4, from the <code>Pattern</code> rule. Try <code>10.0.4.17</code> for a server-side rejection.</div>'}</div>
          </div></div>
        <div class="field-row">
          <div class="field"><label class="lbl" for="f-site">Site</label>
            <select class="input" id="f-site" name="site">${SITES.map(s => `<option${d.site === s ? ' selected' : ''}>${s}</option>`).join('')}</select></div>
          <div class="field"><label class="lbl" for="f-rate">Rate / s</label>
            <input class="input" id="f-rate" value="${d.rate.toFixed(2)}" readonly>
            <div class="hint">Derived from occurrences &divide; duration.</div></div>
        </div>
      </div>
      <div class="panel-foot" style="justify-content:flex-end">
        <a class="btn btn-ghost" href="${PAGES.devices.path}">Cancel</a>
        <button class="btn btn-primary" data-guard="${GUARD}" type="submit">Save</button>
      </div></form>`,
  });
}

export const eventsPanel = d => panel({
  leaf: 'events', title: 'Recent events', ok: true,
  desc: 'A Table of a different model, beside the form. Split constrains nothing about what its children are about.',
  actions: `<a class="iconbtn" href="${devicePath(d.id)}" title="Refresh this leaf" data-act="refresh-leaf">${svg(I.refresh)}</a>`,
  body: `<div class="panel-body"><table>
    <thead><tr><th style="width:70px">Time</th><th style="width:110px">Kind</th><th>Detail</th></tr></thead>
    <tbody>${eventsFor(d.id).map(e => `<tr><td class="mono dim">${e.at}</td><td>${e.kind}</td>
      <td class="${e.detail === 'ok' ? 'dim' : ''}">${e.detail === 'ok' ? 'ok' : `<span style="color:var(--red)">${esc(e.detail)}</span>`}</td></tr>`).join('')}
    </tbody></table></div>`,
});

export const rawPanel = d => panel({
  leaf: 'raw', title: 'Raw', desc: 'The model as the loader returned it.',
  body: `<div class="panel-body pad"><pre style="margin:0;font:12px/1.6 ui-monospace,Menlo,monospace;color:var(--text-2);white-space:pre-wrap">${
    esc(JSON.stringify({ id: d.id, ip: d.ip, site: d.site, occurrences: d.count, duration_s: d.duration, rate: d.rate, status: d.status }, null, 2))}</pre></div>`,
});

// -------------------------------------------------------------------- bodies
export function devicesBody(args = {}) {
  return row('overview', 'Overview', `${DEVICES.length} devices`, `<div class="stack">${devicesPanel(args)}</div>`) +
    note(`
      <p><b>The toolbar is the page's <code>Args</code> struct.</b> A named value that lives in the URL, survives reload and sharing, and reloads whatever reads it. The bar is generated from the argument list — nothing is placed by hand.</p>
      <p><b>Every control here is a real link to a real file.</b> Sort headers, the pager, the row links. Disable JavaScript entirely and the application still works; you lose panel-level refresh and the query-string arguments, nothing else.</p>
      <p><b>The cross-fade between pages is four lines of CSS.</b> <code>@view-transition{navigation:auto}</code> plus a <code>view-transition-name</code> per region. The sidebar is its own snapshot and identical on every page, so it does not appear to move.</p>
      <p class="flag"><b>Three things here the IR cannot express today.</b> Panel titles and descriptions — <code>ir.Form</code> and <code>ir.Table</code> have no <code>Title</code>. The collapsible <b>Overview</b> row — <code>ir.Stack</code> has no <code>Label</code>. And the bar in the Rate column — that one is small, a render hint on <code>ir.Field</code>, not a new node kind.</p>`);
}

export function deviceBody(d, args = {}) {
  const tab = args.tab || 'overview';
  const tabs = [['overview', 'Overview'], ['raw', 'Raw']];
  return `<div class="tabs">${tabs.map(([k, l]) =>
      `<a class="tab${tab === k ? ' active' : ''}" href="${here(devicePath(d.id), args, { tab: k === 'overview' ? '' : k })}">${l}</a>`).join('')}</div>
    <div data-tabbody>${tab === 'overview'
      ? `<div class="split w64">${deviceFormPanel(d, args.__err)}${eventsPanel(d)}</div>`
      : rawPanel(d)}</div>
    ${note(`
      <p><b>The sidebar still shows Devices as active.</b> This page declares <code>Nav.Shadow</code>, so it highlights another entry rather than adding one — the breadcrumb carries the position instead.</p>
      <p><b>The device id is a path argument, so it is a path.</b> This page is a real file at <code>${devicePath(d.id)}</code>. The <code>tab</code> argument is a query argument, so it rides on the same file. That is <code>ArgSpec.InPath</code>, made literal by the filesystem.</p>
      <p><b>Save with IP <code>10.0.4.17</code></b> to see a rejection no rule can catch — uniqueness needs the service, so it comes back as <code>Effect.Fields</code> with a nil error and the panel re-renders with what you typed.</p>`)}`;
}

export function alertsBody(args = {}) {
  return row('alerts-row', 'Alerts', `${ALERTS.filter(a => !a.ack).length} open`, `<div class="stack">${alertsPanel(args)}</div>`) +
    note(`
      <p><b>Flip the role switch to viewer.</b> The bulk actions grey out with a reason — the same <code>Guard</code> that would reject the request gates the control, so there is no second authorisation rule to keep in sync.</p>
      <p><b>Selection is not an argument.</b> It never enters the URL: it is request state, not page state, so a shared link does not carry someone else's selection. Which is why, with no JavaScript, the action bar is a plain form post.</p>`);
}

export const settingsBody = () => `<div class="stack">${panel({
  leaf: 'ingest', title: 'Ingest',
  desc: 'Every constraint below is declared as data and rendered as an HTML attribute.',
  body: `<form data-settings-form method="post" action="${PAGES.ingest.path}" novalidate>
    <div class="panel-body pad"><div class="field-row">
      <div class="field"><label class="lbl" for="s-name">Collector name <span class="req">*</span></label>
        <input class="input" id="s-name" name="name" value="eu-north-1" required minlength="3" maxlength="32">
        <div class="hint">Required, 3–32 characters.</div></div>
      <div class="field"><label class="lbl" for="s-port">Port <span class="req">*</span></label>
        <input class="input" id="s-port" name="port" type="number" value="8125" required min="1" max="65535">
        <div class="hint">A zero lower bound is a real constraint, so bounds are pointers.</div></div>
    </div></div>
    <div class="panel-foot" style="justify-content:flex-end">
      <button class="btn btn-primary" data-guard="${GUARD}" type="submit">Save</button></div></form>`,
})}</div>
${note(`<p><b>Submit with an empty name or a port above 65535.</b> The browser refuses before any request — the same rule set the server re-runs after. One declaration, two enforcements, and the first one needs no script.</p>
  <p><b>The toolbar says this page declares no arguments.</b> Absence is the configuration.</p>`)}`;

export const retentionBody = () => `<div class="stack">${panel({
  leaf: 'retention', title: 'Retention', desc: 'A second page under the same nav group.',
  body: `<form data-settings-form method="post" action="${PAGES.retention.path}" novalidate>
    <div class="panel-body pad"><div class="field">
      <label class="lbl" for="s-retain">Retention (days)</label>
      <input class="input" id="s-retain" name="retain" type="number" value="30" min="0" max="365" style="max-width:220px">
      <div class="hint">Optional. Absent means the default, so a zero stays out of the URL.</div></div></div>
    <div class="panel-foot" style="justify-content:flex-end">
      <button class="btn btn-primary" data-guard="${GUARD}" type="submit">Save</button></div></form>`,
})}</div>
${note(`<p class="flag"><b>This nav group is invented.</b> <code>ir.Nav</code> has <code>Label</code>, <code>Shadow</code> and <code>Hidden</code> — no section and no parent. Grafana's sidebar is two levels deep; webui's is flat. Adding a group is a real decision, not a styling one.</p>`)}`;

export const forbiddenBody = () => `<div class="state-page">${svg(I.lock)}
  <p class="state-title">Not permitted</p>
  <p class="state-desc">The page guard ran before anything was loaded, so this device was never read.</p>
  <a class="btn btn-secondary" href="${PAGES.devices.path}">Back to devices</a></div>`;

export function compileBody() {
  const errs = [
    { where: 'page "/device/{id}"  —  DetailsArgs', detail: 'the path declares {id} but DetailsArgs has no field for it',
      fix: 'Add a field named Id to DetailsArgs, or change the placeholder to match an existing field.' },
    { where: 'page "/alert"  —  AlertArgs', detail: 'two fields both map to the argument "sev"',
      fix: 'Rename one field, or give it a distinct name with a `webui:"..."` tag.' },
    { where: 'page "/settings"  —  SettingsArgs', detail: 'field Window has unsupported type []string',
      fix: 'Arguments must be string, bool, int, int64 or float64 — a URL carries one value per name.' },
  ];
  return `<div class="compile"><h2>Failed to compile</h2>
    <p class="count">${errs.length} problems in this app.</p>
    ${errs.map(e => `<section><div class="where">${esc(e.where)}</div>
      <pre>${esc(e.detail)}</pre><p class="fix"><b>Fix:</b> ${esc(e.fix)}</p></section>`).join('')}
    ${note('<p><b>Every problem at once, each with a fix.</b> The errors are structured rather than scraped from a compiler, so there is no reason to report them one at a time. This page is served at every path under the prefix until the app compiles.</p>')}</div>`;
}

// -------------------------------------------------------------------- shell
export function document_({ title, navId, crumbs, toolbar, body, page, deviceId, method = 'GET', url: shown }) {
  return `<!doctype html>
<html lang="en" data-theme="dark" data-role="editor">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>${esc(title)} — webui</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
<link rel="stylesheet" href="/assets/app.css">
<script src="/assets/prefs.js"></script>
</head>
<body data-page="${esc(page || '')}"${deviceId ? ` data-device-id="${esc(deviceId)}"` : ''}>
<div class="app">
  <aside class="sidebar">
    <div class="side-brand"><div class="logo"><span></span></div><b>Collector</b></div>
    <div class="side-scroll">${navHTML(navId)}</div>
    <div class="side-foot">
      <div class="side-row"><span data-role-label>Role: editor</span>
        <label class="switch"><input type="checkbox" data-pref="role" checked><span class="slider"></span></label></div>
      <div class="side-row"><span>Panel refresh</span>
        <label class="switch"><input type="checkbox" data-pref="leaf" checked><span class="slider"></span></label></div>
      <div class="side-row"><span>Light theme</span>
        <label class="switch"><input type="checkbox" data-pref="theme"><span class="slider"></span></label></div>
    </div>
  </aside>
  <div class="main">
    <header class="topbar">
      <nav class="crumbs">${crumbs}</nav>
      <div class="top-right">
        <div class="search">
          <svg viewBox="0 0 24 24">${I.search}</svg>
          <input data-palette-input placeholder="Search…" autocomplete="off">
          <span class="kbd">&#8984;K</span>
        </div>
        <a class="iconbtn" href="/README.txt" title="How this mock is built">
          <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M9.5 9a2.5 2.5 0 1 1 3.2 2.4c-.7.2-.7.8-.7 1.3M12 17h.01"/></svg></a>
        <div class="avatar">OL</div>
      </div>
      <div class="palette" data-palette></div>
    </header>
    <div class="toolbar" data-toolbar>${toolbar}</div>
    <div class="scroll">${body}</div>
  </div>
</div>
<div class="statusbar">
  <span class="method" data-method>${method}</span>
  <span class="url" data-url>${esc(shown)}</span>
  <span class="netlog" data-netlog><span class="spin"></span><span data-netmsg></span></span>
</div>
<div class="toasts" data-toasts></div>
<script type="module" src="/assets/render.mjs"></script>
<script type="module" src="/assets/enhance.mjs"></script>
</body>
</html>
`;
}
