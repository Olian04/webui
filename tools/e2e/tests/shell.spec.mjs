import { test, expect, watchConsole } from './fixtures.mjs';

test('the theme is chosen with a button and is kept for the next page', async ({ page }) => {
  await page.goto('/admin/');
  const html = page.locator('html');
  const toggle = page.getByRole('switch', { name: /theme/i });
  const before = await html.getAttribute('data-theme');
  await toggle.click();
  const chosen = await html.getAttribute('data-theme');
  expect(chosen).not.toBe(before);
  await page.goto('/admin/device');
  await expect(html).toHaveAttribute('data-theme', chosen);
});

test('the console says once for a tab what built the page, and nothing else', async ({ page }) => {
  const said = [];
  page.on('console', (m) => said.push(`${m.type()}: ${m.text()}`));
  await page.goto('/admin/');
  await page.goto('/admin/device');
  const credit = said.filter((s) => s.includes('webui'));
  expect(credit).toHaveLength(1);
  expect(credit[0].startsWith('info:')).toBe(true);
  expect(said).toHaveLength(1);
});

test('no page of the sidebar says anything wrong in the console', async ({ page }) => {
  const problems = watchConsole(page);
  await page.goto('/admin/');
  const hrefs = await page.getByRole('navigation', { name: 'Main' }).getByRole('link').evaluateAll((as) => as.map((a) => a.getAttribute('href')));
  expect(hrefs.length).toBeGreaterThan(5);
  for (const href of hrefs) {
    await page.goto(href);
    await expect(page.getByRole('main')).toBeVisible();
  }
  expect(problems).toEqual([]);
});

test('a long list scrolls inside its panel and the page does not grow with it', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 600 });
  await page.goto('/admin/layout/list');
  const grew = await page.evaluate(() => document.documentElement.scrollHeight > window.innerHeight + 1);
  expect(grew).toBe(false);
  const many = page.getByRole('heading', { name: 'Many items' }).locator('xpath=ancestor::*[.//ul][1]');
  const scrolls = await many.evaluate((el) => {
    const body = [...el.querySelectorAll('*')].find((n) => n.scrollHeight > n.clientHeight + 1 && getComputedStyle(n).overflowY !== 'visible');
    return !!body;
  });
  expect(scrolls).toBe(true);
});

test('the link in a column header is large enough to hit', async ({ page }) => {
  await page.goto('/admin/device');
  const box = await page.getByRole('table', { name: 'Devices' }).getByRole('link', { name: 'ID' }).boundingBox();
  expect(box.width).toBeGreaterThanOrEqual(24);
  expect(box.height).toBeGreaterThanOrEqual(24);
});

test('two panels of a row are side by side when there is room and one under the other when there is not', async ({ page }) => {
  const top = async (name) => (await page.getByRole('heading', { name }).boundingBox()).y;
  await page.setViewportSize({ width: 1400, height: 900 });
  await page.goto('/admin/layout/split');
  const wide = [await top('Row 1, left: a tall form'), await top('Row 1, right: two rows')];
  expect(Math.abs(wide[0] - wide[1])).toBeLessThan(4);
  await page.setViewportSize({ width: 420, height: 900 });
  const narrow = [await top('Row 1, left: a tall form'), await top('Row 1, right: two rows')];
  expect(Math.abs(narrow[0] - narrow[1])).toBeGreaterThan(40);
});
