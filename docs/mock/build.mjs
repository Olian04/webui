// Writes the site. Every page is a real file; every link points at one.
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import * as V from './assets/views.mjs';
import { DEVICES } from './assets/data.mjs';

const root = './public';
const out = (path, html) => {
  writeFileSync(root + path, html);
  console.log(root + path);
};
const doc = (o) => V.document_(o);

// --- clean the public directory ---
rmSync(root, { recursive: true, force: true });
mkdirSync(root, { recursive: true });

// --- pages whose arguments are all query arguments: one file each ---
out(
  '/index.html',
  doc({
    page: 'devices',
    title: 'Devices',
    navId: 'devices',
    crumbs: V.crumbsHTML([
      ['Collector', '/index.html'],
      ['Devices', null],
    ]),
    toolbar: V.toolbarHTML('devices', '/index.html', {}),
    body: V.devicesBody({}),
    url: '/index.html',
  }),
);

out(
  '/alert.html',
  doc({
    page: 'alerts',
    title: 'Alerts',
    navId: 'alerts',
    crumbs: V.crumbsHTML([
      ['Collector', '/index.html'],
      ['Alerts', null],
    ]),
    toolbar: V.toolbarHTML('alerts', '/alert.html', {}),
    body: V.alertsBody({}),
    url: '/alert.html',
  }),
);

out(
  '/settings.html',
  doc({
    page: 'ingest',
    title: 'Ingest',
    navId: 'ingest',
    crumbs: V.crumbsHTML([
      ['Collector', '/index.html'],
      ['Settings', null],
      ['Ingest', null],
    ]),
    toolbar: V.toolbarHTML('ingest', '/settings.html', {}),
    body: V.settingsBody(),
    url: '/settings.html',
  }),
);

out(
  '/retention.html',
  doc({
    page: 'retention',
    title: 'Retention',
    navId: 'retention',
    crumbs: V.crumbsHTML([
      ['Collector', '/index.html'],
      ['Settings', null],
      ['Retention', null],
    ]),
    toolbar: V.toolbarHTML('retention', '/retention.html', {}),
    body: V.retentionBody(),
    url: '/retention.html',
  }),
);

out(
  '/forbidden.html',
  doc({
    page: 'forbidden',
    title: 'Not permitted',
    navId: 'forbidden',
    crumbs: V.crumbsHTML([
      ['Collector', '/index.html'],
      ['Not permitted', null],
    ]),
    toolbar: '<span class="noargs">This page declares no arguments.</span>',
    body: V.forbiddenBody(),
    method: 'GET',
    url: '/forbidden.html',
  }),
);

out(
  '/compile-error.html',
  doc({
    page: 'compile',
    title: 'Failed to compile',
    navId: 'compile',
    crumbs: '<span class="cur">Compile error</span>',
    toolbar: '<span class="noargs">No app compiled.</span>',
    body: V.compileBody(),
    method: 'GET',
    url: '/compile-error.html',
  }),
);

// --- the path argument is a path, so each device is a file ---
rmSync(root + '/device', { recursive: true, force: true });
mkdirSync(root + '/device', { recursive: true });
for (const d of DEVICES) {
  out(
    V.devicePath(d.id),
    doc({
      page: 'device',
      deviceId: d.id,
      title: d.id,
      navId: 'devices', // Nav.Shadow
      crumbs: V.crumbsHTML([
        ['Collector', '/index.html'],
        ['Devices', '/index.html'],
        [d.id, null],
      ]),
      toolbar: V.toolbarHTML('device', V.devicePath(d.id), {}),
      body: V.deviceBody(d, {}),
      url: V.devicePath(d.id),
    }),
  );
}
console.log(`\n${DEVICES.length + 6} files.`);
