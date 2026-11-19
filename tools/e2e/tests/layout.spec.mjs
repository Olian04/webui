import { test, expect } from './fixtures.mjs';

// How wide and where things are is the stylesheet's, and a Go test reads a page's text and not its pixels. The Activity page has the layouts a page of lists uses (a Split with Tabs
// in it, Columns of unequal weight); these are the ones that can be measured.

test.use({ viewport: { width: 1400, height: 900 } });

// panel is the first panel that a title heads.
const panel = (page, title) => page.getByRole('heading', { name: title, exact: true }).first().locator('xpath=ancestor::*[@data-view][1]');
const width = async (page, title) => (await panel(page, title).boundingBox()).width;

test('a weight is a share of the row: 1 and 2 is a third and two thirds', async ({ page }) => {
  await page.goto('/admin/activity');
  const [calendar, dashboards] = [await width(page, 'Release calendar'), await width(page, 'Dashboards')];
  expect(dashboards / calendar).toBeGreaterThan(1.8);
  expect(dashboards / calendar).toBeLessThan(2.2);
});

test('the panels of a Split are as wide as each other', async ({ page }) => {
  await page.goto('/admin/activity');
  const [alerts, people] = [await width(page, 'Open alerts'), await width(page, 'People')];
  expect(Math.abs(alerts - people)).toBeLessThan(40); // the Tabs the first is in adds nothing to its width
});

test('Tabs in a Split are one column: the strip is over its panel, and the next column is beside them', async ({ page }) => {
  await page.goto('/admin/activity');
  const strip = await page.getByRole('navigation', { name: 'Views: Alerts, Releases' }).boundingBox();
  const shown = await panel(page, 'Open alerts').boundingBox();
  const beside = await panel(page, 'People').boundingBox();
  expect(Math.abs(strip.x - shown.x)).toBeLessThan(2); // the panel is in the strip's column
  expect(shown.y).toBeGreaterThan(strip.y);
  expect(beside.x).toBeGreaterThan(strip.x + strip.width - 1); // and the other column is not under it
});
