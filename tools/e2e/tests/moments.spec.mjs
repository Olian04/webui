import { test, expect } from './fixtures.mjs';

// A moment is read whole: it is not broken at its space, whatever width the table has, and a table that has no room for that scrolls and is not cut. What decides it is
// the content, not a guess at it, so it holds for any Format an app gives.

// how many lines each of these elements is drawn on.
async function lines(table, selector) {
  return table.locator(selector).evaluateAll((els) => els.map((el) => el.getClientRects().length));
}

test.describe('the devices table', () => {
  for (const width of [1400, 1000, 760]) {
    test(`reads a moment on one line at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto('/admin/device');
      const table = page.getByRole('table', { name: 'Devices' });
      const moments = await lines(table, 'tbody td time');
      expect(moments.length).toBeGreaterThan(0);
      expect(moments.every((n) => n === 1)).toBe(true);
    });
  }

  test('scrolls inside its panel, which keeps to the page, when there is no room for the whole of it', async ({ page }) => {
    await page.setViewportSize({ width: 700, height: 900 });
    await page.goto('/admin/device');
    const table = page.getByRole('table', { name: 'Devices' });
    expect((await lines(table, 'tbody td time')).every((n) => n === 1)).toBe(true);
    const panel = await table.locator('xpath=ancestor::div[@data-view][1]').boundingBox();
    const whole = await table.boundingBox();
    expect(whole.width).toBeGreaterThan(panel.width); // the table is wider than its panel: it scrolls there
    expect(panel.x + panel.width).toBeLessThanOrEqual(700); // and the panel is not pushed past the window
    // Something around the table is what scrolls to the rest of it: the first ancestor that moves when it is asked to.
    const scrolls = await table.evaluate((el) => {
      for (let a = el.parentElement; a; a = a.parentElement) {
        a.scrollLeft = 120;
        if (a.scrollLeft > 0) return a.contains(el) && a !== document.documentElement && a !== document.body;
      }
      return false;
    });
    expect(scrolls).toBe(true);
  });
});
