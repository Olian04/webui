import { test, expect } from './fixtures.mjs';

// A phone is up to 640px: the sidebar is a drawer and the top bar's actions are a menu, each a checkbox that is named and a label, so they open without script.

test.use({ viewport: { width: 390, height: 800 } });

test('the sidebar is a drawer that opens and that Escape closes', async ({ page }) => {
  await page.goto('/admin/device');
  const sidebar = page.getByRole('navigation', { name: 'Main' });
  await expect(sidebar).toBeHidden();
  await page.getByRole('checkbox', { name: 'Navigation menu' }).check();
  await expect(sidebar).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(sidebar).toBeHidden();
});

test('the top bar has its search behind a menu, and it is there at 320px', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 700 });
  await page.goto('/admin/device');
  await page.getByRole('checkbox', { name: 'More actions' }).check();
  await expect(page.getByRole('searchbox').or(page.getByRole('combobox', { name: /search/i })).or(page.getByRole('textbox', { name: /search/i })).first()).toBeVisible();
});

test.describe('with no script', () => {
  test.use({ javaScriptEnabled: false, reducedMotion: 'reduce' }); // a toast that is still moving cannot be clicked

  test('the drawer opens on a click of its box', async ({ page }) => {
    await page.goto('/admin/device');
    const sidebar = page.getByRole('navigation', { name: 'Main' });
    await expect(sidebar).toBeHidden();
    await page.getByRole('checkbox', { name: 'Navigation menu' }).check();
    await expect(sidebar).toBeVisible();
  });

  test('a toast is dismissed with a click and is gone', async ({ page }) => {
    await page.goto('/admin/settings');
    await page.getByRole('button', { name: 'Save' }).click();
    const toast = page.getByRole('status').filter({ hasText: 'Saved' });
    await expect(toast).toBeVisible();
    await toast.getByTitle('Dismiss').locator('visible=true').click(); // what a pointer sees: a button, which with no script is the label of a box
    await expect(toast).toBeHidden();
  });
});
