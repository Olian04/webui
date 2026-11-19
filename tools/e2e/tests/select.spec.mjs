import { test, expect } from './fixtures.mjs';

// With script a select is an editable combobox: its chosen values are badges, followed by a text input that narrows the list of options as it is typed in. Without script it is the browser's own
// <select>. The Go tests say what is in the page; here it is what typing does.

test.use({ viewport: { width: 1280, height: 900 } });

const tags = (page) => page.getByRole('combobox', { name: 'Tags' });
const options = (page) => page.getByRole('option');

test.describe('a select of several', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/admin/device/new');
    await tags(page).click();
  });

  test('is narrowed by what is typed, in any case, and says when nothing matches', async ({ page }) => {
    await expect(options(page)).toHaveCount(12);
    await tags(page).fill('GP');
    await expect(options(page)).toHaveCount(1);
    await expect(options(page).first()).toHaveText('gpu');
    await tags(page).fill('o');
    await expect(options(page).filter({ hasText: 'mqtt' })).toHaveCount(0);
    await expect(options(page).filter({ hasText: 'outdoor' })).toHaveCount(1);
    await tags(page).fill('zzz');
    await expect(options(page)).toHaveCount(0);
    await expect(page.getByRole('status').filter({ hasText: 'No option matches' })).toBeVisible();
  });

  test('has the first match ticked by Enter, and is searched again from the whole list', async ({ page }) => {
    await tags(page).fill('gp');
    await page.keyboard.press('Enter');
    await expect(page.getByRole('option', { name: 'gpu' })).toHaveAttribute('aria-selected', 'true');
    await expect(tags(page)).toHaveValue(''); // what was typed is taken back, to look for the next
    await expect(options(page)).toHaveCount(12);
    await tags(page).fill('ed');
    await page.keyboard.press('Enter');
    await expect(page.getByRole('option', { name: 'edge' })).toHaveAttribute('aria-selected', 'true');
  });

  test('has the arrows move over what is left, and Space typed and not a choice', async ({ page }) => {
    await tags(page).fill('o'); // lorawan, modbus, indoor and outdoor
    await expect(options(page)).toHaveCount(4);
    await page.keyboard.press('ArrowDown');
    await page.keyboard.type(' ');
    await expect(tags(page)).toHaveValue('o '); // a space is typed, and is not a choice
    await expect(options(page)).toHaveCount(4);
    await expect(page.getByRole('listbox', { name: 'Tags' }).locator('[aria-selected=true]')).toHaveCount(0);
  });

  test('has Backspace on nothing typed take back the last value, and Escape take back what was typed before it closes', async ({ page }) => {
    await page.getByRole('option', { name: 'gpu' }).click();
    await page.getByRole('option', { name: 'edge' }).click();
    await expect(page.getByRole('option', { name: 'edge' })).toHaveAttribute('aria-selected', 'true');
    await tags(page).focus();
    await page.keyboard.press('Backspace'); // the last of the badges, which are in the order of the options: gpu is after edge
    await expect(page.getByRole('option', { name: 'gpu' })).toHaveAttribute('aria-selected', 'false');
    await expect(page.getByRole('option', { name: 'edge' })).toHaveAttribute('aria-selected', 'true');
    await tags(page).fill('x8');
    await page.keyboard.press('Escape');
    await expect(tags(page)).toHaveValue('');
    await expect(options(page)).toHaveCount(12); // still open, whole again
    await page.keyboard.press('Escape');
    await expect(options(page)).toHaveCount(0);
  });

  test('is sent as what was chosen, whatever was typed', async ({ page }) => {
    await tags(page).fill('x86');
    await page.keyboard.press('Enter');
    await tags(page).fill('mq');
    await page.keyboard.press('Enter');
    await page.keyboard.press('Escape');
    await page.getByRole('textbox', { name: 'IP' }).fill('10.9.9.9');
    await page.getByRole('button', { name: 'Create', exact: true }).click();
    await page.goto('/admin/device?devices.filter.tags=x86');
    await expect(page.getByRole('table', { name: 'Devices' }).getByRole('cell', { name: '10.9.9.9' })).toHaveCount(1);
  });
});

test('a select of one is typed into as well, and closes on a choice', async ({ page }) => {
  await page.goto('/admin/device/new');
  const status = page.getByRole('combobox', { name: 'Status' });
  await status.click();
  await expect(options(page)).not.toHaveCount(0);
  await status.fill('deg');
  await expect(options(page)).toHaveCount(1);
  await page.keyboard.press('Enter');
  await expect(options(page)).toHaveCount(0); // a choice of one closes the list
  await expect(page.locator('.badges').filter({ hasText: 'Degraded' }).first()).toBeVisible();
});

test("a column's filter has a box that narrows its options", async ({ page }) => {
  await page.goto('/admin/device');
  await page.getByRole('table', { name: 'Devices' }).getByLabel('Filter Tags').first().click();
  const find = page.getByRole('searchbox', { name: 'Find an option' });
  const checkbox = (name) => page.getByRole('checkbox', { name, exact: true });
  await expect(checkbox('arm64')).toBeVisible();
  await expect(checkbox('mqtt')).toBeVisible();
  await find.fill('mq');
  await expect(checkbox('mqtt')).toBeVisible();
  await expect(checkbox('arm64')).toBeHidden();
  await find.press('Enter'); // does not send the form that applies the filter
  await expect(page).not.toHaveURL(/devices\.filter\.tags/);
  await find.fill('');
  await expect(checkbox('arm64')).toBeVisible();
});

test.describe('without script', () => {
  test.use({ javaScriptEnabled: false });
  test('a select is the browser\'s own, and has no box to type into', async ({ page }) => {
    await page.goto('/admin/device/new');
    await expect(page.getByRole('listbox', { name: 'Tags' })).toBeVisible();
    await expect(page.getByRole('combobox', { name: 'Tags' })).toHaveCount(0);
    await expect(page.locator('input.msel-input')).toHaveCount(0);
  });
});
