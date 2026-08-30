import { type ConsoleMessage, expect, type Page, test } from "@playwright/test";

// Real-browser regression for the security-header baseline. The Go header
// unit tests pin the exact strings; this spec proves what they actually do
// in a genuine browser: rich content (Mermaid, ZenUML, KaTeX, sortable
// tables, SVG/external images, code blocks) renders without a single CSP
// refusal, while raw-HTML inline event scripts stay unexecuted. Asserting
// rendered outcomes — not header text — is what catches a policy that
// silently breaks the reader.

const richDocumentPath = "/doc/security-rich-content.md";
const rawHTMLDocumentPath = "/doc/security-raw-html.md";

// Chromium reports every CSP refusal on the console; collecting them gives a
// page-wide assertion that is stricter than probing one resource at a time.
function collectCSPRefusals(page: Page): string[] {
  const refusals: string[] = [];
  page.on("console", (message: ConsoleMessage) => {
    if (
      message.type() === "error" &&
      message.text().includes("Content Security Policy")
    ) {
      refusals.push(message.text());
    }
  });
  return refusals;
}

test("renders rich content under the strict CSP without refusals", async ({
  page,
}) => {
  const refusals = collectCSPRefusals(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(richDocumentPath);
  await page.waitForFunction(
    () => document.querySelector(".markdown-body") !== null,
  );

  // Both diagrams (flowchart + ZenUML) finish rendering — a plugin or
  // runtime blocked by the policy degrades into source text or a syntax
  // error long before these selectors pass.
  await page.waitForFunction(
    () =>
      document.querySelectorAll(".m2h-mermaid-frame .mermaid > svg").length ===
      2,
  );
  await expect(page.getByText("Syntax error in text")).toHaveCount(0);

  // KaTeX math is rendered, not left as dollar-sign source.
  await expect(page.locator(".katex").first()).toBeVisible();
  await expect(page.getByText("$E = mc^2$")).toHaveCount(0);

  // The code block carries its enhancement affordance.
  await expect(
    page.getByRole("button", { name: "复制代码" }).first(),
  ).toBeVisible();

  // The local SVG image actually loads — an img-src that forgot 'self'
  // would leave it broken.
  const svgDisplayed = await page.evaluate(() => {
    const image = Array.from(
      document.querySelectorAll<HTMLImageElement>(".markdown-body img"),
    ).find((element) => element.src.endsWith("architecture.svg"));
    return image ? image.complete && image.naturalWidth > 0 : false;
  });
  expect(svgDisplayed).toBe(true);

  // Tablesort enhanced the table (role=columnheader is its stamp) and
  // clicking the numeric header reorders the rows.
  await page.waitForFunction(
    () =>
      document.querySelectorAll(".markdown-body table [role=columnheader]")
        .length >= 2,
  );
  const firstCell = page.locator(".markdown-body tbody tr td").first();
  await expect(firstCell).toHaveText("m2h");
  await page.locator(".markdown-body table [role=columnheader]").nth(1).click();
  await expect(firstCell).toHaveText("Nginx");

  // Nothing on the page — app bundle, runtimes, images, styles — was
  // refused by the policy while all of the above rendered.
  expect(refusals).toEqual([]);
});

test("keeps raw-HTML inline scripts unexecuted", async ({ page }) => {
  const refusals = collectCSPRefusals(page);

  await page.goto(rawHTMLDocumentPath);
  await page.waitForFunction(
    () => document.querySelector(".markdown-body") !== null,
  );

  // The probe image points at a missing path, so its error event fires;
  // without the CSP that is exactly where the inline handler would run.
  // Chromium reports the blocked handler on the console — the refusal is
  // the proof that the CSP, not luck, kept the payload silent.
  await expect.poll(() => refusals.length).toBeGreaterThan(0);

  const xss = await page.evaluate(
    () => (window as unknown as Record<string, unknown>)["__m2h_xss"],
  );
  expect(xss).toBeUndefined();
  const inlineScriptRan = await page.evaluate(
    () =>
      (window as unknown as Record<string, unknown>)["__m2h_xss_inline_script"],
  );
  expect(inlineScriptRan).toBeUndefined();
});

test("serves the hardened header baseline on every route", async ({
  request,
}) => {
  const page = await request.get("/");
  expect(page.status()).toBe(200);
  const headers = page.headers();
  expect(headers["content-security-policy"]).toContain("script-src 'self'");
  expect(headers["x-content-type-options"]).toBe("nosniff");
  expect(headers["referrer-policy"]).toBe("same-origin");
  expect(headers["x-frame-options"]).toBe("SAMEORIGIN");
  expect(headers["permissions-policy"]).toContain("geolocation=()");

  // A 404 response keeps the same baseline — security headers must not be
  // conditional on success.
  const missing = await request.get("/doc/missing-document.md");
  expect(missing.status()).toBe(404);
  expect(missing.headers()["content-security-policy"]).toContain(
    "script-src 'self'",
  );

  // The assets route overrides the CSP with its stricter sandbox policy.
  const svg = await request.get("/assets/images/architecture.svg");
  expect(svg.status()).toBe(200);
  expect(svg.headers()["content-security-policy"]).toBe(
    "sandbox; default-src 'none'",
  );
});
