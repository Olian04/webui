import { test, expect, mark, isKept } from './fixtures.mjs';

// What a visitor with script gets that a page of links does not: a panel is replaced where it stands, and the rest of the page does not move.

test('a sort replaces the table and does not navigate the page', async ({ page }) => {
  await page.goto('/admin/device');
  await mark(page);
  const table = page.getByRole('table', { name: 'Devices' });
  await table.getByRole('columnheader', { name: 'ID' }).getByRole('link', { name: 'ID' }).click();
  await expect(table.getByRole('columnheader', { name: 'ID' })).toHaveAttribute('aria-sort', 'ascending');
  await expect(page).toHaveURL(/devices\.sort=id/);
  expect(await isKept(page)).toBe(true);
});

test('a filter narrows the rows of the table in place', async ({ page }) => {
  await page.goto('/admin/device');
  await mark(page);
  const table = page.getByRole('table', { name: 'Devices' });
  const rows = table.getByRole('row');
  const before = await rows.count();
  await table.getByLabel('Filter ID').first().click();
  await page.getByRole('searchbox', { name: 'Filter ID' }).fill('dev_27c');
  await page.getByRole('searchbox', { name: 'Filter ID' }).press('Enter');
  await expect(page).toHaveURL(/devices\.filter\.id=dev_27c/);
  await expect.poll(async () => rows.count()).toBeLessThan(before);
  expect(await isKept(page)).toBe(true);
});

test('a tab shows its panel in place and the address keeps the choice', async ({ page }) => {
  await page.goto('/admin/layout/tabs');
  await mark(page);
  await expect(page.getByRole('table', { name: 'Open alerts' })).toBeVisible();
  await page.getByRole('navigation', { name: 'Views: Alerts, Devices' }).getByRole('link', { name: 'Devices' }).click();
  await expect(page).toHaveURL(/tabs\.tab=devices/);
  await expect(page.getByRole('table', { name: 'Open alerts' })).toHaveCount(0);
  expect(await isKept(page)).toBe(true);

  // The choice is in the address, so it survives a reload.
  await page.reload();
  await expect(page.getByRole('table', { name: 'Open alerts' })).toHaveCount(0);
});

test('a question that is asked before an action can be answered no', async ({ page }) => {
  await page.goto('/admin/');
  await page.getByRole('button', { name: 'Sync now' }).click();
  const question = page.getByRole('alertdialog');
  await expect(question).toBeVisible();
  await question.getByRole('button', { name: 'Cancel' }).click();
  await expect(question).toBeHidden();
  await expect(page.getByText(/Syncing, \d+ of \d+ devices/)).toHaveCount(0);
});

test('a panel that is not final asks for itself again until it is', async ({ page }) => {
  await page.goto('/admin/');
  await page.getByRole('button', { name: 'Sync now' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Sync now' }).click();
  const progress = page.getByText(/Syncing, \d+ of \d+ devices/);
  await expect(progress).toBeVisible();
  await mark(page); // from here on the page is not navigated: only the panel is asked for again
  const first = await progress.textContent();
  await expect(progress).not.toHaveText(first, { timeout: 8_000 });
  expect(await isKept(page)).toBe(true);
});
