import { test, expect } from './fixtures.mjs';

// A toolbar puts the controls of its right side at the right edge, and the Go tests say what is in it, not where it is. The demo's page of toolbar cases
// (/admin/layout/toolbars) says what each is meant to do; the case of one button at each edge is the simplest to measure.

async function edges(page) {
  const left = page.getByRole('button', { name: 'A button at the left' });
  const right = page.getByRole('button', { name: 'A button at the right' });
  const main = await page.getByRole('main').boundingBox();
  const a = await left.boundingBox();
  const b = await right.boundingBox();
  return { a, b, mainRight: main.x + main.width, mainLeft: main.x };
}

test('a button at each edge is one row on a wide window, at the two edges', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('/admin/layout/toolbars');
  const { a, b, mainLeft, mainRight } = await edges(page);
  expect(Math.abs(a.y - b.y)).toBeLessThan(2);
  expect(a.x - mainLeft).toBeLessThan(40);
  expect(mainRight - (b.x + b.width)).toBeLessThan(40);
});

test('when the two do not fit the right one is in the next row, still at the right edge', async ({ page }) => {
  await page.setViewportSize({ width: 280, height: 800 });
  await page.goto('/admin/layout/toolbars');
  const { a, b, mainRight } = await edges(page);
  expect(b.y).toBeGreaterThan(a.y + a.height - 1);
  expect(mainRight - (b.x + b.width)).toBeLessThan(40);
});
