import { test, expect, watchConsole } from './fixtures.mjs';

// An Embed shows a page of this site in a frame. The Go tests say what is in the markup; here it is what a browser makes of it: the page in the frame is drawn, it carries the visitor's session, it is
// there with script off, and nothing about it is called wrong by the console (a policy that blocks it says so there).

const status = (page) => page.frameLocator('iframe[title="Status board"]');

test('the status board of the System page is a frame of a page of this site, which is drawn', async ({ page }) => {
  const problems = watchConsole(page);
  await page.goto('/admin/system');
  await expect(page.getByRole('heading', { name: 'Status board', level: 2 })).toBeVisible();
  await expect(status(page).getByRole('img', { name: /Collector status: healthy/ })).toBeVisible();
  await expect(page.locator('iframe[title="Status board"]')).not.toHaveAttribute('sandbox', /.*/); // the page is the app's own code
  expect(problems).toEqual([]);
});

test('the page in the frame has the look of the app, from the stylesheet that App.Styles names', async ({ page }) => {
  const problems = watchConsole(page);
  await page.goto('/admin/system');
  const frame = status(page);
  await expect(frame.getByRole('img', { name: /Collector status/ })).toBeVisible();
  const look = (locator, ...props) => locator.evaluate((el, props) => Object.fromEntries(props.map((p) => [p, getComputedStyle(el)[p]])), props);
  // The page has no class anywhere. Its canvas, its text and its font are the app's, and a table is a table of the app: its head is the app's quiet 500 weight, not the browser's bold.
  // The frame sits in a panel, so its ground is the panel's, not the app's canvas behind it.
  const frameBody = await look(frame.locator('body'), 'backgroundColor', 'color', 'fontFamily');
  const hostBody = await look(page.locator('body'), 'backgroundColor', 'color', 'fontFamily');
  expect(frameBody.color).toBe(hostBody.color);
  expect(frameBody.fontFamily).toBe(hostBody.fontFamily);
  expect(frameBody.backgroundColor).not.toBe(hostBody.backgroundColor);
  expect(frameBody.backgroundColor).toBe(await page.locator('main').first().evaluate((el) => { const p = document.createElement('i'); p.style.color = 'var(--panel)'; el.append(p); const c = getComputedStyle(p).color; p.remove(); return c; }));
  expect(await frame.locator('a').first().evaluate((el) => getComputedStyle(el).color)).toBe(await frame.locator('body').evaluate((el) => { const probe = document.createElement('i'); probe.style.color = 'var(--blue-text)'; el.append(probe); return getComputedStyle(probe).color; }));
  const head = await look(frame.getByRole('columnheader').first(), 'fontWeight', 'textTransform');
  expect(head.fontWeight).toBe('500');
  expect(await frame.locator('[class]').count()).toBe(0);
  expect(problems).toEqual([]);
});

test('the page in the frame was asked for with the visitor\'s session', async ({ page, context }) => {
  await context.request.post('/login', { form: { role: 'viewer' } });
  await page.goto('/admin/system');
  await expect(status(page).getByText('the viewer you are signed in as')).toBeVisible();
});

test('a link under the frame opens the page in a tab of its own', async ({ page }) => {
  await page.goto('/admin/system');
  const popup = page.waitForEvent('popup');
  await page.getByRole('link', { name: /Open in a new tab/ }).click();
  expect(new URL((await popup).url()).pathname).toBe('/frame/status');
});

test('a device has its own page in the frame, from the address that Load built', async ({ page }) => {
  await page.goto('/admin/device/dev_27c38b?tabs.tab=location');
  await expect(page.getByRole('heading', { name: 'Site map', level: 2 })).toBeVisible();
  await expect(page.frameLocator('iframe[title="Site map"]').getByRole('img', { name: /dev_27c38b at / })).toBeVisible();
});

test('the frame fits a phone', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 800 });
  await page.goto('/admin/system');
  await expect(status(page).getByRole('img', { name: /Collector status/ })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  expect(overflow).toBe(false);
});

test.describe('without script', () => {
  test.use({ javaScriptEnabled: false });
  test('the frame and the page in it are there all the same', async ({ page }) => {
    await page.goto('/admin/system');
    await expect(page.getByRole('heading', { name: 'Status board', level: 2 })).toBeVisible();
    await expect(status(page).getByRole('img', { name: /Collector status: healthy/ })).toBeVisible();
  });
});
