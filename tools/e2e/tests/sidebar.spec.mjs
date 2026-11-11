import { test, expect, watchConsole } from './fixtures.mjs';

// The sidebar's edge is a handle to drag, or to move with the keys: a window splitter. What it does is the same for a pointer and a key, and it is remembered in this
// browser.

test.use({ viewport: { width: 1400, height: 900 } });

const handleOf = (page) => page.getByRole('separator', { name: 'Resize sidebar' });

// the width of the sidebar, as it is drawn.
async function widthOf(page) {
  return handleOf(page).evaluate((handle) => Math.round(document.getElementById(handle.getAttribute('aria-controls')).getBoundingClientRect().width));
}

test.describe('the sidebar splitter', () => {
  test('is a vertical separator that says its value and its range', async ({ page }) => {
    await page.goto('/admin/device');
    const handle = handleOf(page);
    await expect(handle).toBeVisible();
    await expect(handle).toHaveAttribute('aria-orientation', 'vertical');
    const now = await widthOf(page);
    await expect(handle).toHaveAttribute('aria-valuenow', String(now));
    const [min, max] = [Number(await handle.getAttribute('aria-valuemin')), Number(await handle.getAttribute('aria-valuemax'))];
    expect(min).toBeLessThan(now);
    expect(max).toBeGreaterThan(now);
  });

  test('moves with a drag, by what the pointer moved', async ({ page }) => {
    await page.goto('/admin/device');
    const before = await widthOf(page);
    const box = await handleOf(page).boundingBox();
    const [x, y] = [box.x + box.width / 2, box.y + 200];
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x + 40, y, { steps: 4 });
    await page.mouse.move(x + 60, y, { steps: 4 });
    await page.mouse.up();
    expect(await widthOf(page)).toBe(before + 60);
    await expect(handleOf(page)).toHaveAttribute('aria-valuenow', String(before + 60));
  });

  test('moves with the arrow keys, by 16px, and by 64 with Shift, and goes to its ends with Home and End', async ({ page }) => {
    await page.goto('/admin/device');
    const before = await widthOf(page);
    const handle = handleOf(page);
    await handle.focus();
    await page.keyboard.press('ArrowRight');
    expect(await widthOf(page)).toBe(before + 16);
    await page.keyboard.press('Shift+ArrowRight');
    expect(await widthOf(page)).toBe(before + 16 + 64);
    await page.keyboard.press('ArrowLeft');
    expect(await widthOf(page)).toBe(before + 64);
    await page.keyboard.press('Home');
    expect(await widthOf(page)).toBe(Number(await handle.getAttribute('aria-valuemin')));
    await page.keyboard.press('End');
    expect(await widthOf(page)).toBe(Number(await handle.getAttribute('aria-valuemax')));
    await page.keyboard.press('ArrowRight'); // no further than the end
    expect(await widthOf(page)).toBe(Number(await handle.getAttribute('aria-valuemax')));
  });

  test('goes back to its own width with Enter, and with a double click', async ({ page }) => {
    await page.goto('/admin/device');
    const own = await widthOf(page);
    await handleOf(page).focus();
    await page.keyboard.press('Shift+ArrowRight');
    expect(await widthOf(page)).not.toBe(own);
    await page.keyboard.press('Enter');
    expect(await widthOf(page)).toBe(own);
    await page.keyboard.press('Shift+ArrowRight');
    await handleOf(page).dblclick();
    expect(await widthOf(page)).toBe(own);
  });

  test('is remembered in this browser, and is the same on the next page', async ({ page }) => {
    await page.goto('/admin/device');
    await handleOf(page).focus();
    await page.keyboard.press('Shift+ArrowRight');
    const chosen = await widthOf(page);
    await page.reload();
    expect(await widthOf(page)).toBe(chosen);
    await page.goto('/admin/alert');
    expect(await widthOf(page)).toBe(chosen);
    // and reset forgets it
    await handleOf(page).focus();
    await page.keyboard.press('Enter');
    await page.reload();
    expect(await widthOf(page)).not.toBe(chosen);
  });

  test('is not there on a phone', async ({ page }) => {
    await page.setViewportSize({ width: 600, height: 900 });
    await page.goto('/admin/device');
    await expect(handleOf(page)).toBeHidden();
  });

  test('says nothing wrong on the console', async ({ page }) => {
    const problems = watchConsole(page);
    await page.goto('/admin/device');
    await handleOf(page).focus();
    await page.keyboard.press('ArrowRight');
    expect(problems).toEqual([]);
  });
});
