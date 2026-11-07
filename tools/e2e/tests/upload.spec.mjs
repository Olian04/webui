import { test, expect, mark, isKept } from './fixtures.mjs';

test('a file is sent with its own result, and the history is asked for again', async ({ page }) => {
  await page.goto('/admin/firmware');
  await mark(page);
  const name = `fw-${Date.now()}.bin`;
  await page.locator('input[type=file]').first().setInputFiles({ name, mimeType: 'application/octet-stream', buffer: Buffer.alloc(3000, 7) });
  await page.getByRole('button', { name: 'Upload' }).click();

  // It is answered where it was sent from, with a line of its own and what the server said of it.
  await expect(page.getByRole('listitem').filter({ hasText: name })).toContainText('Received');
  // And the table of what was uploaded, which is another panel, has it without the page being left.
  await expect(page.getByRole('table', { name: 'Upload history' }).getByRole('cell', { name })).toBeVisible();
  expect(await isKept(page)).toBe(true);
});
