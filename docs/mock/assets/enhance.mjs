// The optional enhancement, and the only genuinely client-side script here.
// It changes nothing about what the links are — it changes how much of the page
// is replaced when one is followed.
//
// The rule that decides: a link to a DIFFERENT path is a navigation and is left
// alone (the browser handles it, and @view-transition cross-fades it). A link
// to the SAME path with different arguments belongs to the leaf it sits in, so
// only that leaf is replaced.

import * as V from './views.mjs';
import { deviceById, TAKEN_IPS } from './data.mjs';

const $ = (s) => document.querySelector(s);
const page = document.body.dataset.page;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const pref = (k, d) => {
  try {
    return localStorage.getItem('wui.' + k) ?? d;
  } catch (e) {
    return d;
  }
};
const setPref = (k, v) => {
  try {
    localStorage.setItem('wui.' + k, v);
  } catch (e) {}
};
const leafOn = () => pref('leaf', 'on') === 'on';
const LEAF_COLS = { devices: 6, alerts: 5, events: 3 };

/* ------------------------------- status bar ------------------------------ */
function bar(method, url, note) {
  $('[data-method]').textContent = method;
  $('[data-url]').textContent = url;
  $('[data-netmsg]').textContent = note || '';
  $('[data-netlog]').classList.toggle('on', !!note);
}
function toast(title, desc, kind) {
  const el = document.createElement('div');
  el.className = 'toast' + (kind === 'err' ? ' err' : '');
  el.innerHTML = `<div><div class="toast-title">${V.esc(title)}</div>${desc ? `<div class="toast-desc">${V.esc(desc)}</div>` : ''}</div>`;
  $('[data-toasts]').appendChild(el);
  setTimeout(() => {
    el.classList.add('out');
    setTimeout(() => el.remove(), 220);
  }, 3400);
}

/* ------------------------------ leaf refresh ----------------------------- */
function leafHTML(name, args) {
  if (name === 'devices') return V.devicesPanel(args);
  if (name === 'alerts') return V.alertsPanel(args);
  const d = deviceById(document.body.dataset.deviceId);
  if (name === 'events') return V.eventsPanel(d);
  if (name === 'form') return V.deviceFormPanel(d);
  if (name === 'raw') return V.rawPanel(d);
  return null;
}
async function refreshLeaf(name, href) {
  const el = document.querySelector(`[data-leaf="${name}"]`);
  const args = Object.fromEntries(new URL(href, location.href).searchParams);
  const html = leafHTML(name, args);
  if (!el || !html) {
    location.href = href;
    return;
  }

  const tb = el.querySelector('tbody');
  if (tb) tb.innerHTML = V.skeletonRows(LEAF_COLS[name] || 5);
  bar(
    'GET',
    new URL(href, location.href).pathname + new URL(href, location.href).search,
    `leaf ${name}`,
  );
  history.pushState(null, '', href);
  // In the real app this is a fetch of exactly this URL with a header naming
  // one leaf. Here the renderer runs locally, because http.server cannot vary
  // its response on a query string.
  await sleep(230);
  document.querySelector(`[data-leaf="${name}"]`).outerHTML = leafHTML(
    name,
    args,
  );
  $('[data-toolbar]').innerHTML = V.toolbarHTML(page, location.pathname, args);
  bar('GET', location.pathname + location.search);
}

/* -------------------------------- clicks --------------------------------- */
document.addEventListener('click', async (e) => {
  const rowhead = e.target.closest('.rowhead');
  if (rowhead) {
    rowhead.classList.toggle('closed');
    document
      .querySelector(`[data-rowbody="${rowhead.dataset.row}"]`)
      .classList.toggle('closed');
    return;
  }
  const grp = e.target.closest('[data-group]');
  if (grp) {
    grp.classList.toggle('open');
    document
      .querySelector(`[data-sub="${grp.dataset.group}"]`)
      .classList.toggle('open');
    return;
  }
  if (!e.target.closest('.search') && !e.target.closest('[data-palette]'))
    $('[data-palette]').classList.remove('open');

  const guarded = e.target.closest('[data-guard]');
  if (guarded && document.documentElement.dataset.role === 'viewer') {
    e.preventDefault();
    toast('Not permitted', guarded.dataset.guard, 'err');
    return;
  }

  if (e.target.closest('[data-act="refresh-menu"]')) {
    e.preventDefault();
    toast(
      'Auto-refresh',
      'Not a webui concept — a control plane refreshes a leaf on demand, it does not poll.',
    );
    return;
  }

  const a = e.target.closest('a[href]');
  if (!a || e.metaKey || e.ctrlKey || e.shiftKey || a.target) return;

  const dest = new URL(a.getAttribute('href'), location.href);
  const leaf = a.closest('[data-leaf]');
  // Same path, different arguments, inside a leaf → that leaf reloads.
  // Anything else → an ordinary navigation, cross-faded by CSS.
  if (leafOn() && leaf && dest.pathname === location.pathname) {
    e.preventDefault();
    refreshLeaf(leaf.dataset.leaf, dest.pathname + dest.search);
  }

  const row = e.target.closest('tr.clickable');
  if (row && !e.target.closest('a,input')) location.href = row.dataset.href;
});

/* ------------------------------- selection ------------------------------- */
function syncSelection() {
  const boxes = [
    ...document.querySelectorAll(
      '[data-leaf="alerts"] tbody input[type=checkbox]',
    ),
  ];
  const n = boxes.filter((b) => b.checked).length;
  const label = document.querySelector('[data-selcount]');
  if (label) label.textContent = `${n} selected`;
}
document.addEventListener('change', (e) => {
  if (e.target.matches('[data-pickall]')) {
    document
      .querySelectorAll('[data-leaf="alerts"] tbody input[type=checkbox]')
      .forEach((b) => (b.checked = e.target.checked));
  }
  if (e.target.closest('[data-leaf="alerts"]')) syncSelection();

  const v = e.target.dataset.var;
  if (v && e.target.tagName === 'SELECT') {
    const u = new URL(location.href);
    e.target.value
      ? u.searchParams.set(v, e.target.value)
      : u.searchParams.delete(v);
    u.searchParams.delete('offset');
    const leaf = { devices: 'devices', alerts: 'alerts' }[page];
    if (leafOn() && leaf) refreshLeaf(leaf, u.pathname + u.search);
    else location.href = u.pathname + u.search;
  }

  const p = e.target.dataset.pref;
  if (p === 'theme') {
    setPref('theme', e.target.checked ? 'light' : 'dark');
    document.documentElement.dataset.theme = pref('theme');
  }
  if (p === 'role') {
    setPref('role', e.target.checked ? 'editor' : 'viewer');
    document.documentElement.dataset.role = pref('role');
    document.querySelector('[data-role-label]').textContent =
      'Role: ' + pref('role');
  }
  if (p === 'leaf') {
    setPref('leaf', e.target.checked ? 'on' : 'off');
    toast(
      e.target.checked ? 'Panel refresh on' : 'Panel refresh off',
      e.target.checked
        ? 'A link inside a panel now replaces that panel only.'
        : 'Every link is now an ordinary navigation. The cross-fade is CSS.',
    );
  }
});

/* --------------------------------- forms --------------------------------- */
document.addEventListener('submit', async (e) => {
  const form = e.target;
  if (form.matches('[data-varform]')) return; // a real GET, let it go

  if (!form.matches('[data-device-form],[data-settings-form]')) return;
  e.preventDefault();
  if (!form.checkValidity()) {
    [...form.elements].forEach((el) => {
      if (el.willValidate && !el.checkValidity()) {
        el.classList.add('invalid');
        let m = el.parentElement.querySelector('.err');
        if (!m) {
          m = document.createElement('div');
          m.className = 'err';
          el.parentElement.appendChild(m);
        }
        m.textContent = el.validationMessage;
      }
    });
    toast(
      'Not submitted',
      'The browser rejected it from the declared rules.',
      'err',
    );
    return;
  }
  bar('POST', location.pathname + location.search, 'submit');
  await sleep(380);
  bar('GET', location.pathname + location.search);

  if (form.matches('[data-settings-form]')) {
    toast('Saved', 'Effect returned a toast and stayed on the page.');
    return;
  }

  const d = deviceById(document.body.dataset.deviceId);
  const ip = form.elements.ip.value.trim();
  if (TAKEN_IPS.has(ip) && ip !== d.ip) {
    document.querySelector('[data-leaf="form"]').outerHTML = V.deviceFormPanel(
      { ...d, ip },
      { field: 'ip', message: 'already in use by another device' },
    );
    toast('Could not save', '1 field needs attention.', 'err');
    return;
  }
  toast('Device saved', 'Effect redirected back to Devices.');
  setTimeout(() => {
    location.href = '/index.html';
  }, 500);
});

/* ------------------------------- palette --------------------------------- */
const PAL = [
  ...Object.values(V.PAGES).filter((p) => p.path),
  { path: '/forbidden.html', title: 'Not permitted' },
  { path: '/compile-error.html', title: 'Compile error' },
];
document.addEventListener('input', (e) => {
  if (!e.target.matches('[data-palette-input]')) return;
  const q = e.target.value.toLowerCase();
  const hits = PAL.filter((p) =>
    (p.title + ' ' + (p.group || '')).toLowerCase().includes(q),
  );
  $('[data-palette]').innerHTML =
    '<div class="pal-label">Pages</div>' +
    (hits.length
      ? hits
          .map(
            (p) =>
              `<a class="pal-item" href="${p.path}"><span>${p.group ? p.group + ' / ' : ''}${p.title}</span><span class="mono">${p.path}</span></a>`,
          )
          .join('')
      : '<div class="pal-empty">Nothing matches. The list is the compiled app\'s pages — there is no index to fall out of date.</div>');
  $('[data-palette]').classList.add('open');
});
document.addEventListener('keydown', (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault();
    $('[data-palette-input]').focus();
  }
  if (e.key === 'Escape') $('[data-palette]').classList.remove('open');
});

/* --------------------------- restore switch state ------------------------ */
document.querySelector('[data-pref="theme"]').checked =
  pref('theme', 'dark') === 'light';
document.querySelector('[data-pref="role"]').checked =
  pref('role', 'editor') === 'editor';
document.querySelector('[data-pref="leaf"]').checked = leafOn();
document.querySelector('[data-role-label]').textContent =
  'Role: ' + pref('role', 'editor');
window.addEventListener('popstate', () => location.reload());
syncSelection();
