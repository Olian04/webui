import { test, expect } from './fixtures.mjs';

// A badge is always large enough for its text: as wide as the text when it can be, and, when the room is narrower than that, taller, and never a box with its text
// running out of it.

// put makes a badge in a box of the given width and says where the badge is, and where its text is.
async function put(page, boxWidth, text) {
  return page.evaluate(([boxWidth, text]) => {
    document.querySelectorAll('.probe').forEach((el) => el.remove());
    const box = document.createElement('div');
    box.className = 'probe';
    box.style.cssText = `position:fixed;top:60px;left:20px;width:${boxWidth}px;background:#888`;
    box.innerHTML = `<span class="badge tone-mute"><span class="dot" aria-hidden="true"></span>${text}</span>`;
    document.body.append(box);
    const badge = box.querySelector('.badge').getBoundingClientRect();
    const words = document.createRange();
    words.selectNodeContents(box.querySelector('.badge'));
    const lines = [...words.getClientRects()];
    const inside = lines.every((r) => r.left >= badge.left - 0.5 && r.right <= badge.right + 0.5 && r.top >= badge.top - 0.5 && r.bottom <= badge.bottom + 0.5);
    return { width: badge.width, height: badge.height, box: boxWidth, inside, lines: lines.length };
  }, [boxWidth, text]);
}

test.describe('a badge', () => {
  test('is one line as wide as its text when there is room', async ({ page }) => {
    await page.goto('/admin/files');
    const one = await put(page, 400, 'count size');
    expect(one.inside).toBe(true);
    expect(one.height).toBeLessThan(26);
    expect(one.width).toBeLessThan(150);
  });

  test('holds its text in a box too narrow for it, by growing taller and never wider than the box', async ({ page }) => {
    await page.goto('/admin/files');
    const one = await put(page, 400, 'count size');
    for (const box of [70, 55, 45]) {
      const narrow = await put(page, box, 'count size');
      expect(narrow.inside, `${box}px`).toBe(true);
      expect(narrow.width, `${box}px`).toBeLessThanOrEqual(box + 0.5);
      expect(narrow.height, `${box}px`).toBeGreaterThan(one.height);
    }
  });

  test('holds a word longer than the box', async ({ page }) => {
    await page.goto('/admin/files');
    const long = await put(page, 60, 'extraordinarily');
    expect(long.inside).toBe(true);
    expect(long.width).toBeLessThanOrEqual(60.5);
  });
});
