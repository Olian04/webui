import { test, expect } from './fixtures.mjs';

// How wide and where things are is the stylesheet's, and a Go test reads a page's text and not its pixels. The demo has a page for each layout, and
// says on it what each is meant to do; these are the ones that can be measured.

test.use({ viewport: { width: 1400, height: 900 } });

// panel is the first panel that a title heads.
const panel = (page, title) => page.getByRole('heading', { name: title, exact: true }).first().locator('xpath=ancestor::*[@data-view][1]');
const width = async (page, title) => (await panel(page, title).boundingBox()).width;

test('a weight is a share of the row: 1 and 3 is a quarter and three quarters', async ({ page }) => {
  await page.goto('/admin/layout/weights');
  const [a, b] = [await width(page, 'Alerts: 1'), await width(page, 'Devices: 3')];
  expect(b / a).toBeGreaterThan(2.7);
  expect(b / a).toBeLessThan(3.3);
});

test('25, 50 and 25 is a quarter, a half and a quarter', async ({ page }) => {
  await page.goto('/admin/layout/weights');
  const [a, b] = [await width(page, 'Alerts: 25'), await width(page, 'Devices: 50')];
  expect(b / a).toBeGreaterThan(1.8);
  expect(b / a).toBeLessThan(2.2);
});

test('columns with no weight are as wide as each other', async ({ page }) => {
  await page.setViewportSize({ width: 1800, height: 900 }); // four columns of at least 280px, in a row
  await page.goto('/admin/layout/weights');
  const ws = [await width(page, 'One'), await width(page, 'Two'), await width(page, 'Three'), await width(page, 'Four')];
  for (const w of ws) expect(Math.abs(w - ws[0])).toBeLessThan(2);
});

test('Tabs in a Split are one column: the strip is over its panel, and the next column is beside them', async ({ page }) => {
  await page.goto('/admin/layout/tabs');
  const strip = await page.getByRole('navigation', { name: 'Views: Alerts, Devices' }).boundingBox();
  const shown = await panel(page, 'Open alerts').boundingBox();
  const beside = await page.getByRole('navigation', { name: 'Views: Sites, Status' }).boundingBox();
  expect(Math.abs(strip.x - shown.x)).toBeLessThan(2); // the panel is in the strip's column
  expect(shown.y).toBeGreaterThan(strip.y);
  expect(beside.x).toBeGreaterThan(strip.x + strip.width - 1); // and the other Tabs is not under it
});

test('the columns of tables of the same columns are in the same proportions', async ({ page }) => {
  await page.goto('/admin/layout/weights');
  const shares = async (title) => {
    const heads = await page.getByRole('table', { name: title, exact: true }).first().getByRole('columnheader').all();
    const ws = await Promise.all(heads.map(async (h) => (await h.boundingBox()).width));
    const total = ws.reduce((a, b) => a + b, 0);
    return ws.map((w) => w / total);
  };
  const narrow = await shares('Devices: 3'); // in the three quarters
  const wide = await shares('Devices: 50'); // in the half
  expect(narrow.length).toBe(wide.length);
  narrow.forEach((s, i) => expect(Math.abs(s - wide[i])).toBeLessThan(0.03));
});
