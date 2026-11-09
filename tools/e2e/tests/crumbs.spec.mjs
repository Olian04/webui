import { test, expect } from './fixtures.mjs';

// A trail that is too long for the room the top bar leaves it is shortened from the level below the root: the root is level 0 and the page is level N, and the levels
// from 1 up are replaced by an ellipsis, one more of them at each step, until the trail fits.

// what returns, for each crumb of the trail, whether its label is drawn: a crumb that is replaced is in the page, hidden to the eye, and the first of them is
// drawn as an ellipsis in place of its label.
async function drawn(page) {
  return page.evaluate(() => {
    const nav = document.querySelector('nav[aria-label="Breadcrumb"]');
    const crumbs = [...nav.children].filter((el) => !el.classList.contains('sep'));
    return crumbs.map((el) => {
      const label = document.createRange();
      label.selectNodeContents(el);
      return { text: el.textContent.trim(), drawn: el.getBoundingClientRect().width > 2 && label.getBoundingClientRect().width > 2 };
    });
  });
}

// how many ellipses the trail draws.
function ellipses(page) {
  return page.evaluate(() => {
    const nav = document.querySelector('nav[aria-label="Breadcrumb"]');
    return [...nav.children].filter((el) => getComputedStyle(el, '::after').content.includes('\u2026')).length;
  });
}

// room gives the trail exactly this much width, which is what the window and the actions beside it decide, so that the scan below is in steps of its own.
async function room(page, px) {
  await page.evaluate((px) => {
    const nav = document.querySelector('nav[aria-label="Breadcrumb"]');
    nav.style.flex = 'none';
    nav.style.width = px + 'px';
  }, px);
  await page.waitForTimeout(40);
}

// the levels that are replaced when the trail has this much room: their indices, which are not drawn.
async function replaced(page) {
  const crumbs = await drawn(page);
  return crumbs.map((c, i) => (c.drawn ? -1 : i)).filter((i) => i >= 0);
}

// what a trail does as the room shrinks, whatever decides which levels are replaced: whole when it fits, and then replaced from level 1 up, one more at each
// step, with the root and the page always drawn.
async function shrinkFromWholeToReplaced(page) {
  await page.setViewportSize({ width: 1400, height: 900 });
  await page.goto('/admin/files/logs/2026/10');
  await room(page, 900);
  expect(await replaced(page)).toEqual([]);
  expect(await ellipses(page)).toBe(0);

  let seen = 0; // how many levels are replaced: it only grows as the room narrows
  for (let px = 900; px >= 130; px -= 10) {
    await room(page, px);
    const crumbs = await drawn(page);
    const hidden = await replaced(page);
    // The root and the page are never replaced, and what is replaced is the levels from 1 up, without a gap.
    expect(crumbs[0].drawn, `${px}px`).toBe(true);
    expect(crumbs[crumbs.length - 1].drawn, `${px}px`).toBe(true);
    expect(hidden, `${px}px`).toEqual(Array.from({ length: hidden.length }, (_, i) => i + 1));
    expect(hidden.length, `${px}px`).toBeGreaterThanOrEqual(seen);
    seen = hidden.length;
    // The ellipsis is there when something is replaced, and only then, and there is one.
    expect(await ellipses(page), `${px}px`).toBe(hidden.length > 0 ? 1 : 0);
  }
  expect(seen).toBe(3); // at the narrowest, the root, the ellipsis and the page
}

// where the last crumb that is drawn ends, against where the actions begin.
async function neverUnderTheActions(page) {
  for (const w of [1400, 1000, 760, 700, 400]) {
    await page.setViewportSize({ width: w, height: 900 });
    await page.goto('/admin/files/logs/2026/10');
    await page.waitForTimeout(60);
    const { end, start } = await page.evaluate(() => {
      const nav = document.querySelector('nav[aria-label="Breadcrumb"]');
      const ends = [...nav.children].map((el) => el.getBoundingClientRect()).filter((r) => r.width > 2).map((r) => r.right);
      return { end: Math.max(...ends), start: document.querySelector('.top-right').getBoundingClientRect().left };
    });
    // On a phone the actions are a menu, and the trail has the width of the bar.
    expect(end).toBeLessThanOrEqual(w <= 640 ? w : start + 1);
  }
}

// the levels that are replaced are still in the page, as links.
async function replacedLevelsAreStillLinks(page) {
  await page.setViewportSize({ width: 700, height: 900 });
  await page.goto('/admin/files/logs/2026/10');
  const nav = page.getByRole('navigation', { name: 'Breadcrumb' });
  for (const name of ['Files', 'logs', '2026']) await expect(nav.getByRole('link', { name })).toHaveCount(1);
}

// labels longer than the stylesheet takes a level to be: the trail is too wide for its room before the guess replaces anything, and what gives way is the levels
// between the ends, which are cut with an ellipsis, while the root and the page are read whole, as long as anything between them is left to cut.
async function longLabelsGiveWayInTheMiddle(page) {
  await page.setViewportSize({ width: 1400, height: 900 });
  await page.goto('/admin/files/logs/2026/10');
  const labels = ['Collector platform', 'Infrastructure files', 'application-logs', 'archive-2026-q3', 'rotation-ten'];
  await page.evaluate((labels) => {
    const nav = document.querySelector('nav[aria-label="Breadcrumb"]');
    [...nav.children].filter((el) => !el.classList.contains('sep')).forEach((el, i) => { el.textContent = labels[i]; });
  }, labels);
  for (const px of [900, 600, 500, 400, 320]) {
    await room(page, px);
    const state = await page.evaluate(() => {
      const nav = document.querySelector('nav[aria-label="Breadcrumb"]');
      const crumbs = [...nav.children].filter((el) => !el.classList.contains('sep'));
      const ends = [crumbs[0], crumbs[crumbs.length - 1]];
      return {
        overflows: nav.scrollWidth > nav.clientWidth + 1,
        endsCut: ends.some((el) => el.scrollWidth > el.clientWidth + 1),
        reach: Math.max(...[...nav.children].map((el) => el.getBoundingClientRect()).filter((r) => r.width > 2).map((r) => r.right)) - nav.getBoundingClientRect().right,
      };
    });
    expect(state.overflows, `${px}px`).toBe(false);
    expect(state.reach, `${px}px`).toBeLessThanOrEqual(1);
    expect(state.endsCut, `${px}px`).toBe(false);
  }
}

test.describe('a trail of five levels, with script', () => {
  test('is whole when it fits, and is replaced from level 1 up as the room shrinks', ({ page }) => shrinkFromWholeToReplaced(page));
  test('never runs under the actions on its right', ({ page }) => neverUnderTheActions(page));
  test('keeps the levels that were replaced in the page for a reader', ({ page }) => replacedLevelsAreStillLinks(page));
  test('with labels that are long, gives way in the middle', ({ page }) => longLabelsGiveWayInTheMiddle(page));
});

test('with script, the trail keeps the levels that the guess gave up where the labels are short', async ({ page, browser, baseURL }) => {
  // The stylesheet's guess is a width for a level; script has the real ones. The demo's labels are short, so at some room the guess replaces more than it needs.
  const bare = await browser.newContext({ baseURL, javaScriptEnabled: false });
  await bare.request.post('/login', { form: { role: 'editor' } });
  const plain = await bare.newPage();
  let better = 0;
  for (const p of [page, plain]) {
    await p.setViewportSize({ width: 1400, height: 900 });
    await p.goto('/admin/files/logs/2026/10');
  }
  for (let px = 600; px >= 130; px -= 10) {
    await room(page, px);
    await room(plain, px);
    const [withScript, without] = [(await replaced(page)).length, (await replaced(plain)).length];
    expect(withScript, `${px}px`).toBeLessThanOrEqual(without);
    if (withScript < without) better++;
  }
  await bare.close();
  expect(better).toBeGreaterThan(0);
});

// The stylesheet alone is the baseline: what a visitor gets before script has run, or without it.
test.describe('a trail of five levels, without script', () => {
  test.use({ javaScriptEnabled: false });
  test('is whole when it fits, and is replaced from level 1 up as the room shrinks', ({ page }) => shrinkFromWholeToReplaced(page));
  test('never runs under the actions on its right', ({ page }) => neverUnderTheActions(page));
  test('keeps the levels that were replaced in the page for a reader', ({ page }) => replacedLevelsAreStillLinks(page));
  test('with labels that are long, gives way in the middle', ({ page }) => longLabelsGiveWayInTheMiddle(page));
});
