import { test, expect } from './fixtures.mjs';

// A panel's head may carry the buttons that the app declared for the panel as a whole. The demo's Alerts panel has "Silence info": it asks first and does it, as a toolbar's
// button does. The Go tests say what is in the head and what posts; here it is whether it works in a browser, and fits a phone.

const panel = (page) => page.getByRole('heading', { name: 'Alerts', level: 2 }).locator('xpath=ancestor::*[@data-view][1]');

test('a button in the head asks, is done, and the page shows what is left', async ({ page }) => {
  await page.goto('/admin/alert');
  const button = panel(page).getByRole('button', { name: 'Silence info', exact: true });
  await expect(button).toBeVisible();
  await button.click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Silence info' }).click();
  await expect(page.getByText(/Silenced \d+/).first()).toBeVisible();
  await expect(panel(page).getByText('alt_005')).toHaveCount(0); // an info alert is gone
  await expect(panel(page).getByText('alt_003').first()).toBeVisible(); // a critical one is not
});

test('the head of a phone-sized panel takes a second row and does not scroll sideways', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 800 });
  await page.goto('/admin/alert');
  await expect(panel(page).getByRole('button', { name: 'Silence info', exact: true })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  expect(overflow).toBe(false);
});

test.describe('without script', () => {
  test.use({ javaScriptEnabled: false });
  test('it posts and the page is drawn again', async ({ page }) => {
    await page.goto('/admin/alert');
    await panel(page).getByRole('button', { name: 'Silence info', exact: true }).click();
    await page.getByRole('alertdialog').getByRole('button', { name: 'Silence info' }).click();
    await expect(page).toHaveURL(/\/admin\/alert/);
    await expect(page.getByText(/Silenced \d+/).first()).toBeVisible();
  });
});
