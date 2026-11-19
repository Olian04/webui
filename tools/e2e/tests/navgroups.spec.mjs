import { test, expect } from './fixtures.mjs';

// A section of the sidebar is a category that opens and shuts. It is open when it holds the page on screen, what the visitor opens is not kept, and it works without script, since
// it is a disclosure of the page's own. A rail shows every entry, whether the category is open or not.

test.use({ viewport: { width: 1400, height: 900 } });

const nav = (page) => page.getByRole('navigation', { name: 'Main' });
const category = (page, name) => nav(page).getByText(name, { exact: true });
const entry = (page, name) => nav(page).getByRole('link', { name, exact: true });

test.describe('the sidebar', () => {
  test('has open the category that holds the page on screen, and shut the others', async ({ page }) => {
    await page.goto('/admin/device');
    await expect(entry(page, 'Devices')).toBeVisible();
    await expect(entry(page, 'Alerts')).toBeVisible(); // the same category
    await expect(entry(page, 'System')).toBeHidden(); // Operations is shut
    await expect(entry(page, 'Audit log')).toBeHidden();
    await expect(category(page, 'Operations')).toBeVisible(); // and its name is there to open it
  });

  test('opens a category on a click and shuts it on another', async ({ page }) => {
    await page.goto('/admin/device');
    await category(page, 'Operations').click();
    await expect(entry(page, 'System')).toBeVisible();
    await expect(entry(page, 'Devices')).toBeVisible(); // the first is still open
    await category(page, 'Operations').click();
    await expect(entry(page, 'System')).toBeHidden();
    // and the category that holds the page can be shut too
    await category(page, 'Platform').click();
    await expect(entry(page, 'Devices')).toBeHidden();
  });

  test('opens and shuts with the keys of a button', async ({ page }) => {
    await page.goto('/admin/device');
    await nav(page).locator('summary').filter({ hasText: 'Operations' }).focus(); // the category itself, which is the control
    await page.keyboard.press('Enter');
    await expect(entry(page, 'System')).toBeVisible();
    await page.keyboard.press('Space');
    await expect(entry(page, 'System')).toBeHidden();
  });

  test('has open, on the next page, the category that holds that page, and not the one that was opened by hand', async ({ page }) => {
    await page.goto('/admin/device');
    await category(page, 'Operations').click();
    await expect(entry(page, 'System')).toBeVisible();
    await entry(page, 'Alerts').click();
    await expect(page).toHaveURL(/\/admin\/alert/);
    await expect(entry(page, 'Alerts')).toBeVisible();
    await expect(entry(page, 'System')).toBeHidden(); // what was opened is not kept
    await page.goto('/admin/system');
    await expect(entry(page, 'System')).toBeVisible(); // the one that holds the page is open
    await expect(entry(page, 'Devices')).toBeHidden();
  });

  test('shows a rail every entry of every category, open or not', async ({ page }) => {
    await page.goto('/admin/device');
    await expect(entry(page, 'System')).toBeHidden();
    await page.locator('[data-sidebar-toggle]').click(); // the rail
    await expect(entry(page, 'System')).toBeVisible();
    await expect(entry(page, 'Audit log')).toBeVisible();
    await expect(entry(page, 'Devices')).toBeVisible();
    await page.locator('[data-sidebar-toggle]').click();
    await expect(entry(page, 'System')).toBeHidden(); // and the categories are as they were
  });

  test('is a drawer on a phone, with the same categories', async ({ page }) => {
    await page.setViewportSize({ width: 600, height: 900 });
    await page.goto('/admin/device');
    await page.getByRole('checkbox', { name: 'Navigation menu' }).check();
    await expect(entry(page, 'Devices')).toBeVisible();
    await expect(entry(page, 'System')).toBeHidden();
    await category(page, 'Operations').click();
    await expect(entry(page, 'System')).toBeVisible();
  });
});

test.describe('the sidebar, without script', () => {
  test.use({ javaScriptEnabled: false });
  test('opens and shuts its categories all the same', async ({ page }) => {
    await page.goto('/admin/device');
    await expect(entry(page, 'Devices')).toBeVisible();
    await expect(entry(page, 'System')).toBeHidden();
    await category(page, 'Operations').click();
    await expect(entry(page, 'System')).toBeVisible();
  });
});
