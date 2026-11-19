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
