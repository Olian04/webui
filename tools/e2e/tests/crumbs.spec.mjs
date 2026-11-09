import { test, expect } from './fixtures.mjs';

// A trail that is too long for the room the top bar leaves it is shortened from the level below the root: the root is level 0 and the page is level N, and the levels
// from 1 up are replaced by an ellipsis, one more of them at each step, until the trail fits.

// what returns, for each crumb of the trail, whether it is drawn: a crumb that is replaced is in the page, hidden to the eye.
async function drawn(page) {
  return page.evaluate(() => {
    const nav = document.querySelector('nav[aria-label="Breadcrumb"]');
    const crumbs = [...nav.children].filter((el) => !el.classList.contains('sep'));
    return crumbs.map((el) => ({ text: el.textContent.trim(), drawn: el.getBoundingClientRect().width > 2 }));
  });
}

test.describe('a trail of five levels', () => {
  test('is whole when it fits, and is replaced from level 1 up as the room shrinks', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });
    await page.goto('/admin/files/logs/2026/10');
    const nav = page.getByRole('navigation', { name: 'Breadcrumb' });
    expect((await drawn(page)).every((c) => c.drawn)).toBe(true);
    await expect(nav.getByText('…')).toHaveCount(0);

    let seen = 0; // how many levels are replaced: it only grows as the window narrows
    let replacedSomething = false;
    for (let w = 1400; w >= 680; w -= 40) { // down to a phone, at which the actions are a menu and the trail has the room again
      await page.setViewportSize({ width: w, height: 900 });
      await page.waitForTimeout(60);
      const crumbs = await drawn(page);
      const hidden = crumbs.map((c, i) => (c.drawn ? -1 : i)).filter((i) => i >= 0);
      // The root and the page are never replaced, and what is replaced is the levels from 1 up, without a gap.
      expect(crumbs[0].drawn).toBe(true);
      expect(crumbs[crumbs.length - 1].drawn).toBe(true);
      expect(hidden).toEqual(Array.from({ length: hidden.length }, (_, i) => i + 1));
      expect(hidden.length).toBeGreaterThanOrEqual(seen);
      seen = hidden.length;
      replacedSomething ||= hidden.length > 0;
      // The ellipsis is there when something is replaced, and only then.
      await expect(nav.getByText('…')).toHaveCount(hidden.length > 0 ? 1 : 0);
    }
    expect(replacedSomething).toBe(true);
  });

  test('never runs under the actions on its right', async ({ page }) => {
    for (const w of [1400, 1000, 760, 700]) {
      await page.setViewportSize({ width: w, height: 900 });
      await page.goto('/admin/files/logs/2026/10');
      await page.waitForTimeout(60);
      // Where the last crumb that is drawn ends, and where the actions begin.
      const { end, start } = await page.evaluate(() => {
        const nav = document.querySelector('nav[aria-label="Breadcrumb"]');
        const ends = [...nav.children].map((el) => el.getBoundingClientRect()).filter((r) => r.width > 2).map((r) => r.right);
        return { end: Math.max(...ends), start: document.querySelector('.top-right').getBoundingClientRect().left };
      });
      expect(end).toBeLessThanOrEqual(start + 1);
    }
  });

  test('keeps the levels that were replaced in the page for a reader', async ({ page }) => {
    await page.setViewportSize({ width: 700, height: 900 });
    await page.goto('/admin/files/logs/2026/10');
    const nav = page.getByRole('navigation', { name: 'Breadcrumb' });
    for (const name of ['Files', 'logs', '2026']) await expect(nav.getByRole('link', { name })).toHaveCount(1);
  });
});
