// The dataset. Deterministic, because every page is generated independently and
// they have to agree. In the real app this is whatever the loaders return.

function lcg(seed){ let s = seed >>> 0; return () => (s = (s * 1664525 + 1013904223) >>> 0) / 4294967296; }
const r = lcg(20260928);
const pick = (a, n) => a[n % a.length];

export const SITES = ['Stockholm', 'Malmö', 'Göteborg'];
export const STATUSES = ['healthy', 'degraded', 'quiet'];

export const DEVICES = Array.from({ length: 37 }, (_, i) => {
  const count = 12 + Math.floor(r() * 2388);
  const duration = 30 + Math.floor(r() * 60);
  return {
    id: 'dev_' + (0x100000 + i * 0x1d3f7).toString(16).slice(0, 6),
    ip: `10.0.${4 + (i % 5)}.${2 + Math.floor(r() * 248)}`,
    count, duration,
    rate: +(count / duration).toFixed(2),
    status: count > 1500 ? 'healthy' : count > 400 ? 'degraded' : 'quiet',
    site: pick(SITES, i),
  };
});

export const MAXRATE = Math.max(...DEVICES.map(d => d.rate));
export const TAKEN_IPS = new Set(['10.0.4.17', '10.0.5.99']);

export const ALERTS = Array.from({ length: 14 }, (_, i) => ({
  id: 'alr_' + (1000 + i),
  device: DEVICES[i % DEVICES.length].id,
  severity: pick(['critical', 'warning', 'info'], i),
  message: pick([
    'Packet loss above threshold', 'Rate dropped to zero',
    'Firmware behind by 2 versions', 'Clock drift detected',
  ], i),
  ack: i % 5 === 0,
}));

export const eventsFor = () => Array.from({ length: 7 }, (_, i) => ({
  at: `12:${String(40 - i * 3).padStart(2, '0')}`,
  kind: pick(['report', 'handshake', 'retry', 'report', 'report', 'handshake', 'report'], i),
  detail: i === 2 ? 'timeout after 2s' : 'ok',
}));

export const deviceById = id => DEVICES.find(d => d.id === id);
