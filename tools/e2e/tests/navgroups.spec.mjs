import { test, expect } from './fixtures.mjs';

// A section of the sidebar is a category that opens and shuts, and the browser keeps one of them open at a time (they share a name). It is open when it holds the page on screen, what the visitor opens is not kept, and it works without script, since
// it is a disclosure of the page's own. There is no way to narrow the sidebar to icons: it is as wide as its owner dragged it, or a drawer on a phone.

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

  test('opens a category on a click, which shuts the one that was open, and shuts it on another', async ({ page }) => {
    await page.goto('/admin/device');
    await category(page, 'Operations').click();
    await expect(entry(page, 'System')).toBeVisible();
    await expect(entry(page, 'Devices')).toBeHidden(); // only one is open at a time: the first was shut by the browser
    await category(page, 'Operations').click();
    await expect(entry(page, 'System')).toBeHidden();
    // and the one that holds the page can be opened again, which shuts the other
    await category(page, 'Platform').click();
    await expect(entry(page, 'Devices')).toBeVisible();
    await category(page, 'Operations').click();
    await expect(entry(page, 'Devices')).toBeHidden();
    await expect(entry(page, 'System')).toBeVisible();
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
    await entry(page, 'Audit log').click();
    await expect(page).toHaveURL(/\/admin\/audit/);
    await expect(entry(page, 'Audit log')).toBeVisible(); // the category that holds the page
    await expect(entry(page, 'Devices')).toBeHidden();
    await category(page, 'Platform').click(); // opened by hand, which shuts Operations
    await expect(entry(page, 'Devices')).toBeVisible();
    await entry(page, 'Devices').click();
    await expect(page).toHaveURL(/\/admin\/device/);
    await expect(entry(page, 'Devices')).toBeVisible();
    await expect(entry(page, 'System')).toBeHidden(); // what was opened is not kept
    await page.goto('/admin/system');
    await expect(entry(page, 'System')).toBeVisible(); // the one that holds the page is open
    await expect(entry(page, 'Devices')).toBeHidden();
  });

  test('cannot be narrowed to icons, and has no control that says it can', async ({ page }) => {
    await page.goto('/admin/device');
    await expect(page.getByRole('button', { name: /collapse/i })).toHaveCount(0);
    await expect(nav(page).getByRole('link', { name: 'Devices', exact: true })).toContainText('Devices');
  });

  test('is a drawer on a phone, with the same categories', async ({ page }) => {
    await page.setViewportSize({ width: 600, height: 900 });
    await page.goto('/admin/device');
    await page.getByRole('checkbox', { name: 'Navigation menu' }).check();
    await expect(entry(page, 'Devices')).toBeVisible();
    await expect(entry(page, 'System')).toBeHidden();
    await category(page, 'Operations').click();
    await expect(entry(page, 'System')).toBeVisible();
    await expect(entry(page, 'Devices')).toBeHidden(); // the browser keeps one open, with no script
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
    await expect(entry(page, 'Devices')).toBeHidden(); // the browser keeps one open, with no script
  });
});
