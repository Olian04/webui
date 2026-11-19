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

// The choice is the opposite of what the system says, so that it is one that was made: and the sign-in page is in it, and drawn in it (the light theme's
// page is lighter than the dark one's).
for (const system of ['light', 'dark']) {
  test(`the sign-in page follows the theme that was chosen in the app, against a ${system} system`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: system });
    await page.goto('/admin/');
    await page.getByRole('switch', { name: /theme/i }).click();
    const chosen = await page.locator('html').getAttribute('data-theme');
    expect(chosen).not.toBe(system);
    await page.goto('/login');
    await expect(page.locator('html')).toHaveAttribute('data-theme', chosen);
    const red = await page.evaluate(() => Number(getComputedStyle(document.body).backgroundColor.match(/\d+/)[0]));
    expect(chosen === 'light' ? red > 200 : red < 80).toBe(true);
  });
}

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
  // every entry, the ones in a category that is shut included: they are there to be opened
  const hrefs = await page.getByRole('navigation', { name: 'Main' }).locator('a[href]').evaluateAll((as) => as.map((a) => a.getAttribute('href')));
  expect(hrefs.length).toBeGreaterThan(5);
  for (const href of hrefs) {
    await page.goto(href);
    await expect(page.getByRole('main')).toBeVisible();
  }
  expect(problems).toEqual([]);
});

test('a long list is held to the window and scrolls inside its panel', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 400 });
  await page.goto('/admin/activity?open-alerts.limit=11'); // all eleven open alerts, which are more than 16rem holds
  const panel = page.getByRole('heading', { name: 'Open alerts', level: 2 }).locator('xpath=ancestor::*[@data-view][1]');
  const box = await panel.boundingBox();
  expect(box.height).toBeLessThan(400); // held to the window, where eleven items would be taller
  const scrolls = await panel.evaluate((el) => {
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
  await page.goto('/admin/activity');
  const wide = [await top('Release calendar'), await top('Dashboards')];
  expect(Math.abs(wide[0] - wide[1])).toBeLessThan(4);
  await page.setViewportSize({ width: 420, height: 900 });
  const narrow = [await top('Release calendar'), await top('Dashboards')];
  expect(Math.abs(narrow[0] - narrow[1])).toBeGreaterThan(40);
});
