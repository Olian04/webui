import { test, expect, watchConsole, mark, isKept } from './fixtures.mjs';

// A moment is read in UTC, with its zone said, and with script a click on one shows every moment on the page in the visitor's own time zone, which is then remembered in
// this browser. Tokyo is UTC+9 all year, so that what the visitor sees does not depend on the day the suite is run.

test.use({ viewport: { width: 1400, height: 900 }, timezoneId: 'Asia/Tokyo' });

const UTC = /^\d{4}-\d\d-\d\d \d\d:\d\d UTC$/;
const TOKYO = /^\d{4}-\d\d-\d\d \d\d:\d\d (GMT\+9|JST)$/;

const devices = (page) => page.getByRole('table', { name: 'Devices' });
const moments = (page) => devices(page).getByRole('button', { name: /^\d{4}-\d\d-\d\d \d\d:\d\d / });

// what every <time> of the page holds, which is UTC and is never touched.
const held = (page) => page.locator('time[datetime]').evaluateAll((els) => els.map((el) => el.getAttribute('datetime')).filter((at) => !at.includes('.')));

test.describe('a moment', () => {
  test('is read in UTC, with its zone, and is a button that says what it does', async ({ page }) => {
    await page.goto('/admin/device');
    const first = moments(page).first();
    await expect(first).toHaveText(UTC);
    await expect(first).toHaveAttribute('title', /Click for your time zone/);
    expect(await moments(page).count()).toBeGreaterThan(5);
  });

  test('a click shows every moment of the page in the visitor\'s time zone, and a second goes back', async ({ page }) => {
    const problems = watchConsole(page);
    await page.goto('/admin/device');
    const before = await held(page);
    const count = await moments(page).count();
    await moments(page).first().click();
    for (const text of await moments(page).allTextContents()) expect(text.trim()).toMatch(TOKYO);
    expect(await held(page)).toEqual(before); // the moment is what it was
    // 11:30 UTC is 20:30 in Tokyo
    await expect(moments(page).first()).toContainText(/\d\d:\d\d/);
    const first = (await moments(page).first().textContent()).trim();
    expect(first).toMatch(/^2026-10-09 (20|19|18|17|16|15|14|13|12|11):\d\d /);
    await expect(moments(page).first()).toHaveAttribute('title', /Click for UTC/);
    await moments(page).first().click();
    for (const text of await moments(page).allTextContents()) expect(text.trim()).toMatch(UTC);
    expect(await moments(page).count()).toBe(count);
    expect(problems).toEqual([]);
  });

  test('is in the zone that was chosen on the next page and after a reload, and in UTC once that was chosen again', async ({ page }) => {
    await page.goto('/admin/device');
    await moments(page).first().click();
    await page.reload();
    await expect(moments(page).first()).toHaveText(TOKYO);
    await page.goto('/admin/device/dev_27c38b');
    const lastSeen = page.getByRole('button', { name: TOKYO });
    await expect(lastSeen).toHaveText('2026-10-09 20:30 ' + (await lastSeen.textContent()).trim().slice(17));
    await lastSeen.click();
    await expect(page.getByRole('button', { name: UTC })).toHaveText('2026-10-09 11:30 UTC');
    await page.goto('/admin/device');
    await expect(moments(page).first()).toHaveText(UTC);
  });

  test('is switched by the keys, and the change is said', async ({ page }) => {
    await page.goto('/admin/device');
    const first = moments(page).first();
    await first.focus();
    await page.keyboard.press('Enter');
    await expect(first).toHaveText(TOKYO);
    await expect(page.getByRole('status').filter({ hasText: 'Times are shown in your time zone' })).toHaveCount(1);
    await page.keyboard.press('Space');
    await expect(first).toHaveText(UTC);
    await expect(page.getByRole('status').filter({ hasText: 'Times are shown in UTC' })).toHaveCount(1);
  });

  test('in a row that opens a page is a switch, and the row is still opened by a click beside it', async ({ page }) => {
    await page.goto('/admin/device');
    await moments(page).first().click();
    await expect(page).toHaveURL(/\/admin\/device$/); // the click was the moment's, not the row's
    // beside the moment, in the same cell: a row is one link over all of its cells, and is clicked where it is
    const cell = await moments(page).first().locator('xpath=ancestor::td[1]').boundingBox();
    await page.mouse.click(cell.x + 3, cell.y + cell.height / 2);
    await expect(page).toHaveURL(/\/admin\/device\/dev_/);
  });

  test('stays in the zone that was chosen when its panel is fetched again', async ({ page }) => {
    await page.goto('/admin/device');
    await moments(page).first().click();
    await mark(page);
    await devices(page).getByRole('link', { name: 'Last seen' }).click();
    await expect(page).toHaveURL(/sort=/);
    expect(await isKept(page)).toBe(true); // the panel was replaced, and the page was not
    for (const text of await moments(page).allTextContents()) expect(text.trim()).toMatch(TOKYO);
  });

  test('that an app worded with a Format is left as it wrote it, with both moments as its title', async ({ page }) => {
    await page.goto('/admin/alert');
    const alerts = page.getByRole('table', { name: /Alerts/ });
    await expect(alerts.getByText('just now').first()).toHaveAttribute('title', /^2026-10-09 11:30 UTC · 2026-10-09 20:30 (GMT\+9|JST)$/);
    await page.goto('/admin/device');
    await moments(page).first().click();
    await page.goto('/admin/alert');
    await expect(alerts.getByText('just now').first()).toBeVisible(); // the words are the app's, in every zone
  });
});

test.describe('a moment, for a visitor in UTC', () => {
  test.use({ timezoneId: 'UTC' });
  test('reads the same in their own zone, which is UTC', async ({ page }) => {
    await page.goto('/admin/device');
    const first = moments(page).first();
    const text = (await first.textContent()).trim();
    await first.click();
    await expect(first).toHaveText(text);
  });
});

test.describe('a moment, without script', () => {
  test.use({ javaScriptEnabled: false });
  test('is read in UTC, with its zone, and is no button', async ({ page }) => {
    await page.goto('/admin/device');
    await expect(devices(page).getByRole('button', { name: /UTC/ })).toHaveCount(0);
    const text = await devices(page).locator('time[datetime]').first().textContent();
    expect(text.trim()).toMatch(UTC);
  });
  test('holds the moment of what an app worded in its title, in UTC', async ({ page }) => {
    await page.goto('/admin/alert');
    await expect(page.getByRole('table', { name: /Alerts/ }).getByText('just now').first()).toHaveAttribute('title', '2026-10-09 11:30 UTC');
  });
});
