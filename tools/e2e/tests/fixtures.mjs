import { test as base, expect } from '@playwright/test';

// Every scenario is an editor, who may do everything: the demo's /login starts a session as the role that it is asked for.
export const test = base.extend({
  page: async ({ page, context }, use) => {
    const login = await context.request.post('/login', { form: { role: 'editor' } });
    expect(login.ok()).toBeTruthy();
    await use(page);
  },
});

export { expect };

// problems collects what the console and the page say that is wrong: an error, a warning, an exception that nothing caught.
export function watchConsole(page) {
  const problems = [];
  page.on('console', (m) => {
    if (m.type() === 'error' || m.type() === 'warning') problems.push(`${m.type()}: ${m.text()}`);
  });
  page.on('pageerror', (e) => problems.push(`exception: ${e.message}`));
  return problems;
}

// kept marks the document, so that a scenario can say afterwards that the page was not navigated: a navigation makes a new document, and the mark is gone.
export async function mark(page) {
  await page.evaluate(() => { window.__kept = true; });
}

export async function isKept(page) {
  return page.evaluate(() => window.__kept === true);
}
