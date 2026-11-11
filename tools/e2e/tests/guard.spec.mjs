import { test, expect } from './fixtures.mjs';

// A form that the visitor can save is not left with what they changed in it without their say: the browser asks first. A form that nothing was changed in, or that was changed and
// changed back, is left as any page is, and so is a form that was saved or cancelled, which is the visitor leaving it of their own accord.

const DEVICE = '/admin/device/dev_27c38b';

// watching says what the browser asked, and answers it: accept lets the page go, and not lets it stay.
function watching(page, accept) {
  const asked = [];
  page.on('dialog', async (dialog) => {
    asked.push(dialog.type());
    if (accept) await dialog.accept();
    else await dialog.dismiss();
  });
  return asked;
}

const ip = (page) => page.getByRole('textbox', { name: 'IP' });
const sidebarLink = (page) => page.getByRole('link', { name: 'Alerts', exact: true });

test.describe('a form that can be saved', () => {
  test('is left without a question when nothing was changed', async ({ page }) => {
    const asked = watching(page, false);
    await page.goto(DEVICE);
    await sidebarLink(page).click();
    await expect(page).toHaveURL(/\/admin\/alert/);
    expect(asked).toEqual([]);
  });

  test('asks before a link takes away what was changed, and keeps it when the visitor stays', async ({ page }) => {
    const asked = watching(page, false);
    await page.goto(DEVICE);
    await ip(page).click();
    await ip(page).fill('10.9.9.9');
    await sidebarLink(page).click();
    await expect.poll(() => asked).toEqual(['beforeunload']);
    await expect(page).toHaveURL(/\/admin\/device\/dev_27c38b/);
    await expect(ip(page)).toHaveValue('10.9.9.9');
  });

  test('lets the visitor leave when they say so', async ({ page }) => {
    const asked = watching(page, true);
    await page.goto(DEVICE);
    await ip(page).click();
    await ip(page).fill('10.9.9.9');
    await sidebarLink(page).click();
    await expect(page).toHaveURL(/\/admin\/alert/);
    expect(asked).toEqual(['beforeunload']);
  });

  test('asks before a reload, and before Back', async ({ page }) => {
    const asked = watching(page, false);
    await page.goto('/admin/alert');
    await page.goto(DEVICE);
    await ip(page).click();
    await ip(page).fill('10.9.9.9');
    // the visitor stays, so neither is done, and neither is waited for
    await page.evaluate(() => setTimeout(() => location.reload()));
    await expect.poll(() => asked.length).toBe(1);
    await page.evaluate(() => setTimeout(() => history.back()));
    await expect.poll(() => asked.length).toBe(2);
    expect(new Set(asked)).toEqual(new Set(['beforeunload']));
    await expect(ip(page)).toHaveValue('10.9.9.9');
  });

  test('is left without a question when what was changed was changed back', async ({ page }) => {
    const asked = watching(page, false);
    await page.goto(DEVICE);
    const own = await ip(page).inputValue();
    await ip(page).click();
    await ip(page).fill('10.9.9.9');
    await ip(page).fill(own);
    await sidebarLink(page).click();
    await expect(page).toHaveURL(/\/admin\/alert/);
    expect(asked).toEqual([]);
  });

  test('is left by its own Cancel without a question', async ({ page }) => {
    const asked = watching(page, false);
    await page.goto(DEVICE);
    await ip(page).click();
    await ip(page).fill('10.9.9.9');
    await page.getByRole('link', { name: 'Cancel' }).click();
    await expect(page).not.toHaveURL(/dev_27c38b$/);
    expect(asked).toEqual([]);
  });

  test('is saved without a question, and a refusal that draws it again with what was typed is a form with changes', async ({ page }) => {
    const asked = watching(page, false);
    await page.goto(DEVICE);
    await ip(page).click();
    await ip(page).fill('not an address');
    // the browser would refuse a value that is not an address before the form was sent, and it is the server's refusal that is the case here
    await page.locator('form[data-warn-leave]').evaluate((form) => { form.noValidate = true; });
    const refusal = page.waitForResponse((r) => r.request().method() === 'POST');
    await page.getByRole('button', { name: 'Save' }).click();
    expect((await refusal).status()).toBe(422);
    // refused, and drawn again with what was typed, and the visitor was not asked on the way: this is the page that the refusal made, and not the one that was sent
    await expect(page.locator('form[data-edited]')).toHaveCount(1);
    await expect(ip(page)).toHaveValue('not an address');
    expect(asked).toEqual([]);
    await sidebarLink(page).click();
    await expect.poll(() => asked).toEqual(['beforeunload']); // what was typed is only here
    await expect(ip(page)).toHaveValue('not an address');
  });

  test('is not guarded for a visitor who may not save it', async ({ browser, baseURL }) => {
    const context = await browser.newContext({ baseURL });
    const login = await context.request.post('/login', { form: { role: 'viewer' } });
    expect(login.ok()).toBeTruthy();
    const page = await context.newPage();
    const asked = watching(page, false);
    await page.goto(DEVICE);
    await expect(page.locator('form[data-warn-leave]')).toHaveCount(0);
    await sidebarLink(page).click();
    await expect(page).toHaveURL(/\/admin\/alert/);
    expect(asked).toEqual([]);
    await context.close();
  });

  test('does not make a search the visitor typed a form to lose', async ({ page }) => {
    const asked = watching(page, false);
    await page.goto('/admin/device');
    await page.getByRole('combobox', { name: /search/i }).first().click();
    await page.keyboard.type('dev_27');
    await sidebarLink(page).click();
    await expect(page).toHaveURL(/\/admin\/alert/);
    expect(asked).toEqual([]);
  });
});

test.describe('a code editor that can be saved', () => {
  test('asks before what was typed in it is lost', async ({ page }) => {
    const asked = watching(page, false);
    await page.goto('/admin/config');
    const editor = page.locator('.monaco-editor').first();
    await expect(editor).toBeVisible({ timeout: 20000 });
    await editor.click();
    await page.keyboard.type('# a change\n');
    await sidebarLink(page).click();
    await expect.poll(() => asked).toEqual(['beforeunload']);
  });
});
