import { test, expect } from './fixtures.mjs';

// A page that is sent as it is built says that the rest is coming, and takes it back when the last panel has arrived: the status bar's "loading" is
// there from the first byte and gone when the page is whole, and then the bar says how long it took. (The Go tests say what is in the bytes and in what
// order; whether "loading" is still on screen is the stylesheet's.)

test('a page that was sent as it was built is not still loading when it is whole', async ({ page }) => {
  await page.goto('/admin/loading/order');
  await expect(page.getByRole('heading', { name: 'Last (50 ms)' })).toBeVisible();
  await expect(page.getByText(/^\d+(\.\d+)? ms$/).first()).toBeVisible();
  await expect(page.getByText('loading', { exact: true })).toBeHidden();
});
