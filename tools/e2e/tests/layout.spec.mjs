import { test, expect } from './fixtures.mjs';

// How wide and where things are is the stylesheet's, and a Go test reads a page's text and not its pixels. The landing page has the layouts a dashboard uses (Columns of unequal
// weight, a Split with Tabs in it), and these are the ones that can be measured.

test.use({ viewport: { width: 1400, height: 900 } });

// panel is the first panel that a title heads.
const panel = (page, title) => page.getByRole('heading', { name: title, exact: true }).first().locator('xpath=ancestor::*[@data-view][1]');
const width = async (page, title) => (await panel(page, title).boundingBox()).width;

test('a weight is a share of the row: 2 and 1 is two thirds and a third', async ({ page }) => {
  await page.goto('/admin/');
  const [devices, alerts] = [await width(page, 'Devices'), await width(page, 'Open alerts')];
  expect(devices / alerts).toBeGreaterThan(1.8);
  expect(devices / alerts).toBeLessThan(2.2);
});

test('the panels of a Split are as wide as each other', async ({ page }) => {
  await page.goto('/admin/');
  const [events, settings] = [await width(page, 'Recent events'), await width(page, 'Collector settings')];
  expect(Math.abs(events - settings)).toBeLessThan(40); // the Tabs the first is in adds nothing to its width
});

test('Tabs in a Split are one column: the strip is over its panel, and the next column is beside them', async ({ page }) => {
  await page.goto('/admin/');
  const strip = await page.getByRole('navigation', { name: 'Views: Events, Status' }).boundingBox();
  const shown = await panel(page, 'Recent events').boundingBox();
  const beside = await panel(page, 'Collector settings').boundingBox();
  expect(Math.abs(strip.x - shown.x)).toBeLessThan(2); // the panel is in the strip's column
  expect(shown.y).toBeGreaterThan(strip.y);
  expect(beside.x).toBeGreaterThan(strip.x + strip.width - 1); // and the other column is not under it
});

test('the columns of a table are in the same proportions whatever the width of its panel', async ({ page }) => {
  const shares = async () => {
    const heads = await page.getByRole('table', { name: 'Devices', exact: true }).first().getByRole('columnheader').all();
    const ws = await Promise.all(heads.map(async (h) => (await h.boundingBox()).width));
    const total = ws.reduce((a, b) => a + b, 0);
    return ws.map((w) => w / total);
  };
  await page.goto('/admin/');
  const narrow = await shares();
  await page.setViewportSize({ width: 1800, height: 900 });
  const wide = await shares();
  expect(narrow.length).toBe(wide.length);
  narrow.forEach((s, i) => expect(Math.abs(s - wide[i])).toBeLessThan(0.03));
});
