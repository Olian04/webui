// Stands in for the server. A static file server cannot vary its response on a
// query string, so when one is present this re-renders the affected leaves from
// the same view module the build script used. In the real app this file does
// not exist: Go does this, once, on the way out.

import * as V from './views.mjs';
import { deviceById } from './data.mjs';

const $ = s => document.querySelector(s);
const page = document.body.dataset.page;
const args = Object.fromEntries(new URLSearchParams(location.search));
const path = location.pathname;

function swap(sel, html) { const el = $(sel); if (el) el.outerHTML = html; }

if (location.search) {
  $('[data-toolbar]').innerHTML = V.toolbarHTML(page, path, args);
  if (page === 'devices') swap('[data-leaf="devices"]', V.devicesPanel(args));
  if (page === 'alerts')  swap('[data-leaf="alerts"]',  V.alertsPanel(args));
  if (page === 'device') {
    const d = deviceById(document.body.dataset.deviceId);
    $('[data-tabbody]').outerHTML = `<div data-tabbody>${(args.tab || 'overview') === 'raw'
      ? V.rawPanel(d) : `<div class="split w64">${V.deviceFormPanel(d)}${V.eventsPanel(d)}</div>`}</div>`;
    document.querySelectorAll('.tab').forEach(t => t.classList.toggle('active',
      new URL(t.href).searchParams.get('tab') === (args.tab === 'raw' ? 'raw' : null)));
  }
}

$('[data-url]').textContent = path + location.search;
