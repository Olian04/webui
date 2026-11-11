import { test, expect, watchConsole, mark, isKept } from './fixtures.mjs';

// A column's header has a splitter at its edge, to drag or to move with the keys. What a visitor gives a column is its width, and only that column changes: the others keep
// theirs, and a table that is wider than its panel scrolls there. What was given is kept for the table in this browser, and a column goes back to its own width on Enter or a
// double click.

test.use({ viewport: { width: 1400, height: 900 } });

const table = (page) => page.getByRole('table', { name: 'Devices' });
const splitterOf = (page, column) => table(page).getByRole('separator', { name: `Resize ${column} column` });

// the width of a column, as it is drawn: that of its header.
const widthOf = (splitter) => splitter.evaluate((el) => el.parentElement.getBoundingClientRect().width);

// the widths of every header of the table, in order.
const widths = (page) => table(page).locator('thead th').evaluateAll((ths) => ths.map((th) => th.getBoundingClientRect().width));

// drag moves a splitter by dx pixels with the pointer.
async function drag(page, splitter, dx) {
  const box = await splitter.boundingBox();
  const [x, y] = [box.x + box.width / 2, box.y + box.height / 2];
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx / 2, y, { steps: 4 });
  await page.mouse.move(x + dx, y, { steps: 4 });
  await page.mouse.up();
}

test.describe('the columns of a table', () => {
  test('each have a splitter, named for the column, and the selection and the actions have none', async ({ page }) => {
    await page.goto('/admin/alert');
    const alerts = page.getByRole('table', { name: /Alerts/ });
    // the alerts have a selection, a column of actions on the right, and five columns of data
    await expect(alerts.getByRole('separator')).toHaveCount(5);
    await expect(alerts.getByRole('separator', { name: 'Resize Raised column' })).toBeVisible();
    await page.goto('/admin/device');
    const stored = splitterOf(page, 'Stored');
    await expect(stored).toHaveAttribute('aria-orientation', 'vertical');
    const [now, min, max] = [Math.round(await widthOf(stored)), Number(await stored.getAttribute('aria-valuemin')), Number(await stored.getAttribute('aria-valuemax'))];
    await expect(stored).toHaveAttribute('aria-valuenow', String(now));
    expect(min).toBeLessThan(now);
    expect(max).toBeGreaterThan(now);
  });

  test('change only the one that is dragged', async ({ page }) => {
    await page.goto('/admin/device');
    const before = await widths(page);
    const stored = splitterOf(page, 'Stored');
    const own = await widthOf(stored);
    await drag(page, stored, 80);
    const after = await widths(page);
    const at = (await table(page).locator('thead th').evaluateAll((ths) => ths.findIndex((th) => th.textContent.includes('Stored'))));
    expect(Math.abs(after[at] - (own + 80))).toBeLessThan(1.5);
    after.forEach((w, i) => {
      if (i !== at) expect(Math.abs(w - before[i]), `column ${i}`).toBeLessThan(1.5);
    });
    // the table is wider than it was, by what the column grew, and so than its panel: it scrolls there
    const panel = await table(page).locator('xpath=ancestor::div[@data-view][1]').boundingBox();
    expect((await table(page).boundingBox()).width).toBeGreaterThan(panel.width);
  });

  test('are moved by the keys, by 16px and by 64 with Shift, between their least and their most', async ({ page }) => {
    await page.goto('/admin/device');
    const stored = splitterOf(page, 'Stored');
    await stored.focus();
    const start = await widthOf(stored);
    await page.keyboard.press('ArrowRight');
    expect(Math.abs((await widthOf(stored)) - (start + 16))).toBeLessThan(1.5);
    await page.keyboard.press('Shift+ArrowRight');
    expect(Math.abs((await widthOf(stored)) - (start + 80))).toBeLessThan(1.5);
    await page.keyboard.press('ArrowLeft');
    expect(Math.abs((await widthOf(stored)) - (start + 64))).toBeLessThan(1.5);
    await expect(stored).toHaveAttribute('aria-valuenow', String(Math.round(await widthOf(stored))));
    await page.keyboard.press('Home');
    expect(Math.round(await widthOf(stored))).toBe(Number(await stored.getAttribute('aria-valuemin')));
    await page.keyboard.press('End');
    expect(Math.round(await widthOf(stored))).toBe(Number(await stored.getAttribute('aria-valuemax')));
  });

  test('go back to their own width, and only that one, on Enter and on a double click', async ({ page }) => {
    await page.goto('/admin/device');
    const [stored, rate] = [splitterOf(page, 'Stored'), splitterOf(page, 'Rate / s')];
    const own = [await widthOf(stored), await widthOf(rate)];
    await stored.focus();
    await page.keyboard.press('Shift+ArrowRight');
    await rate.focus();
    await page.keyboard.press('Shift+ArrowRight');
    await stored.focus();
    await page.keyboard.press('Enter');
    expect(Math.abs((await widthOf(stored)) - own[0])).toBeLessThan(1.5);
    expect(Math.abs((await widthOf(rate)) - (own[1] + 64))).toBeLessThan(1.5); // the other keeps what it was given
    await page.keyboard.press('Shift+ArrowRight');
    await stored.dblclick();
    expect(Math.abs((await widthOf(stored)) - own[0])).toBeLessThan(1.5);
    expect(Math.abs((await widthOf(rate)) - (own[1] + 64))).toBeLessThan(1.5);
  });

  test('keep what they were given in this browser, for that table', async ({ page }) => {
    const problems = watchConsole(page);
    // another table's column of the same name, to see that it is not moved
    const rate = () => page.getByRole('table').first().getByRole('separator', { name: 'Resize Rate / s column' });
    await page.goto('/admin/compare');
    const other = await widthOf(rate());
    await page.goto('/admin/device');
    const own = await widthOf(splitterOf(page, 'Stored'));
    await splitterOf(page, 'Stored').focus();
    await page.keyboard.press('Shift+ArrowRight');
    await page.reload();
    expect(Math.abs((await widthOf(splitterOf(page, 'Stored'))) - (own + 64))).toBeLessThan(1.5);
    await page.goto('/admin/compare');
    expect(Math.abs((await widthOf(rate())) - other)).toBeLessThan(1.5);
    // and forgetting it forgets it
    await page.goto('/admin/device');
    await splitterOf(page, 'Stored').focus();
    await page.keyboard.press('Enter');
    await page.reload();
    expect(Math.abs((await widthOf(splitterOf(page, 'Stored'))) - own)).toBeLessThan(1.5);
    expect(problems).toEqual([]);
  });

  test('keep their widths when the panel is fetched again for a sort', async ({ page }) => {
    await page.goto('/admin/device');
    await splitterOf(page, 'Stored').focus();
    await page.keyboard.press('Shift+ArrowRight');
    const chosen = Math.round(await widthOf(splitterOf(page, 'Stored')));
    await mark(page);
    await table(page).getByRole('link', { name: 'ID' }).first().click();
    await expect(page).toHaveURL(/sort=/);
    expect(await isKept(page)).toBe(true); // the panel was replaced, and the page was not
    expect(Math.abs((await widthOf(splitterOf(page, 'Stored'))) - chosen)).toBeLessThan(1.5);
  });

  test('ignore widths that were kept for a table with other columns', async ({ page }) => {
    await page.goto('/admin/device');
    const own = await widthOf(splitterOf(page, 'ID'));
    const key = await table(page).getAttribute('data-sizes');
    await page.evaluate((key) => localStorage.setItem('webui.columns', JSON.stringify({ [key]: { n: 3, w: { 0: 500, 1: 500 } } })), key);
    await page.reload();
    expect(Math.abs((await widthOf(splitterOf(page, 'ID'))) - own)).toBeLessThan(1.5);
    // and widths for as many columns as it has are used
    const n = await table(page).getByRole('separator').count();
    await page.evaluate(([key, n]) => localStorage.setItem('webui.columns', JSON.stringify({ [key]: { n, w: { 0: 500 } } })), [key, n]);
    await page.reload();
    expect(Math.round(await widthOf(splitterOf(page, 'ID')))).toBe(500);
  });

  test('have no splitters on a phone, and keep no widths there', async ({ page }) => {
    await page.goto('/admin/device');
    await splitterOf(page, 'Stored').focus();
    await page.keyboard.press('End');
    await page.setViewportSize({ width: 600, height: 900 });
    await page.reload();
    await expect(table(page).getByRole('separator')).toHaveCount(0);
    const stored = await table(page).locator('thead th').evaluateAll((ths) => ths.find((th) => th.textContent.includes('Stored')).getBoundingClientRect().width);
    expect(stored).toBeLessThan(400);
  });
});
