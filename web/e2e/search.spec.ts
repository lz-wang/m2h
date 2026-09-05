import { expect, type Page, test } from "@playwright/test";

// Real-browser regressions for the workspace full-text search: the shortcut
// entry (the only one since the top-bar search button was removed), the
// deep-link into a matched section, and the same-document navigation that
// must not refetch the document.

const DIALOG = '[data-slot="dialog-content"]';
const SEARCH_INPUT = 'input[aria-label="全文搜索"]';
const DEMO_RESULT = "全文搜索演示，search-demo.md";

async function openDialogWithShortcut(page: Page): Promise<void> {
  // The app is client-rendered: wait for the toolbar (and with it the
  // window keydown listener) to exist before pressing the shortcut. The
  // shortcut is the search's only entry — the removed top-bar button must
  // never come back as a ready signal here.
  await expect(page.locator(".reader-toolbar")).toBeVisible();
  await page.keyboard.press("Control+k");
  await expect(page.locator(DIALOG)).toBeVisible();
}

test("Ctrl+K opens the global search dialog", async ({ page }) => {
  await page.goto("/doc/README.md");
  await openDialogWithShortcut(page);
  await expect(page.getByRole("searchbox", { name: "全文搜索" })).toBeVisible();

  // Escape closes it again through the dialog's own dismissal path.
  await page.keyboard.press("Escape");
  await expect(page.locator(DIALOG)).toBeHidden();
});

test("a full-text hit deep-links into the matched section", async ({
  page,
}) => {
  await page.goto("/doc/README.md");
  await openDialogWithShortcut(page);

  await page.fill(SEARCH_INPUT, "unique-backend-token");
  const result = page.getByRole("button", { name: DEMO_RESULT });
  await expect(result).toBeVisible();
  await result.click();

  await expect(page).toHaveURL(/\/doc\/search-demo\.md#backend-search$/);
  const heading = page.getByRole("heading", { name: "Backend Search" });
  await expect(heading).toBeVisible();
  await expect
    .poll(() =>
      heading.evaluate((node) => {
        const box = node.getBoundingClientRect();
        return box.top >= 0 && box.top < window.innerHeight;
      }),
    )
    .toBe(true);
});

test("same-document hits navigate in place without refetching", async ({
  page,
}) => {
  // The frontend token only exists in the frontend section, so opening the
  // document on the backend section and jumping to the frontend hit keeps
  // this a pure same-document navigation.
  let documentRequests = 0;
  page.on("request", (request) => {
    if (request.url().includes("/api/document?path=search-demo.md")) {
      documentRequests += 1;
    }
  });

  await page.goto("/doc/search-demo.md");
  await expect(
    page.getByRole("heading", { name: "Backend Search" }),
  ).toBeVisible();
  expect(documentRequests).toBe(1);

  await openDialogWithShortcut(page);
  await page.fill(SEARCH_INPUT, "unique-frontend-token");
  const result = page.getByRole("button", { name: DEMO_RESULT });
  await expect(result).toBeVisible();
  await result.click();

  await expect(page).toHaveURL(/#frontend-search$/);
  await expect(
    page.getByRole("heading", { name: "Frontend Search" }),
  ).toBeVisible();
  // The document itself was never reloaded for the same-document jump.
  expect(documentRequests).toBe(1);
});
