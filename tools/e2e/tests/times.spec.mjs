import { test, expect, watchConsole } from './fixtures.mjs';

// A moment is in UTC, with its zone said, for every visitor: the browser's time zone does not change it, and there is nothing to click. Tokyo is UTC+9 all year, so that a
// moment that was converted would read differently whatever day the suite is run.

test.use({ viewport: { width: 1400, height: 900 }, timezoneId: 'Asia/Tokyo' });

const devices = (page) => page.getByRole('table', { name: 'Devices' });

test.describe('a moment', () => {
  test('is read in UTC, with its zone, whatever the time zone of the browser is', async ({ page }) => {
    const problems = watchConsole(page);
    await page.goto('/admin/device');
    const moments = await devices(page).locator('time[datetime]').evaluateAll((els) => els.map((el) => ({ at: el.getAttribute('datetime'), text: el.textContent.trim() })));
    expect(moments.length).toBeGreaterThan(5);
    for (const { at, text } of moments) {
      expect(text).toBe(at.slice(0, 10) + ' ' + at.slice(11, 16) + ' UTC'); // 11:30Z is 11:30 UTC, and not 20:30
    }
    expect(problems).toEqual([]);
  });

  test('is not a control: there is nothing to click, and a click does not change it', async ({ page }) => {
    await page.goto('/admin/device');
    await expect(devices(page).getByRole('button', { name: /UTC/ })).toHaveCount(0);
    const first = devices(page).locator('time[datetime]').first();
    const text = await first.textContent();
    const box = await first.boundingBox();
    await page.mouse.click(box.x + 10, box.y + box.height / 2); // in a row that opens a page: the click is the row's, which is one link over its cells
    await expect(page).toHaveURL(/\/admin\/device\/dev_/);
    await page.goBack();
    await expect(devices(page).locator('time[datetime]').first()).toHaveText(text);
  });

  test('that an app worded with a Format is left as it wrote it, and holds the moment in its title, in UTC', async ({ page }) => {
    await page.goto('/admin/alert');
    const raised = page.getByRole('table', { name: /Alerts/ }).getByText('just now').first();
    await expect(raised).toHaveAttribute('title', '2026-10-09 11:30 UTC');
  });
});
