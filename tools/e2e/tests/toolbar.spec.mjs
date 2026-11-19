import { test, expect } from './fixtures.mjs';

// A toolbar puts the controls of its right side at the right edge, and the Go tests say what is in it, not where it is. The Devices page's toolbar is the case of one control at each
// edge, which is the simplest to measure: a link to make a device at the left, and a button to sync them at the right.

async function edges(page) {
  const left = page.getByRole('link', { name: 'Create device' });
  const right = page.getByRole('button', { name: 'Sync now' });
  const main = await page.getByRole('main').boundingBox();
  const a = await left.boundingBox();
  const b = await right.boundingBox();
  return { a, b, mainRight: main.x + main.width, mainLeft: main.x };
}

test('a button at each edge is one row on a wide window, at the two edges', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('/admin/device');
  const { a, b, mainLeft, mainRight } = await edges(page);
  expect(Math.abs(a.y - b.y)).toBeLessThan(2);
  expect(a.x - mainLeft).toBeLessThan(40);
  expect(mainRight - (b.x + b.width)).toBeLessThan(40);
});

test('when the two do not fit the right one is in the next row, still at the right edge', async ({ page }) => {
  await page.setViewportSize({ width: 200, height: 800 });
  await page.goto('/admin/device');
  const { a, b, mainRight } = await edges(page);
  expect(b.y).toBeGreaterThan(a.y + a.height - 1);
  expect(mainRight - (b.x + b.width)).toBeLessThan(40);
});
