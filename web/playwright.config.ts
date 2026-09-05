import { defineConfig, devices } from "@playwright/test";

// Real-browser regression suite. Vitest/jsdom owns the logic tests; Playwright
// owns the behaviors that need a genuine layout engine — scroll restoration
// across a reload while an async table enhancement reflows the body is
// invisible to jsdom, which computes no geometry at all.
//
// The suite runs against the real `m2h` document server: the webServer
// command builds the WebUI, compiles the Go binary with the webui tag, and
// serves the fixture directory under web/e2e/docs.
const port = 8873;

export default defineConfig({
  testDir: "./e2e",
  // One preview server, one fixture document: workers would only fight over it.
  workers: 1,
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI ? "github" : "list",
  // The only screenshot baseline is a text-free strip of solid colors at the
  // sidebar boundary (see tree-stress.spec.ts), which renders identically on
  // every platform — drop the default -darwin/-linux suffix so one committed
  // baseline serves CI and local runs alike.
  snapshotPathTemplate: "{testDir}/{testFileName}-snapshots/{arg}{ext}",
  use: {
    baseURL: `http://127.0.0.1:${port}`,
  },
  // Four engines, four jobs: desktop Chromium owns the full suite (the
  // touch-drag regressions drive CDP, which exists only there), desktop
  // WebKit runs the wheel-driven mobile scroll smoke (mobile WebKit cannot
  // wheel), and the two phone profiles re-run the sidebar focus + first-swipe
  // contracts with the hasTouch/isMobile emulation a desktop viewport never
  // exercises — the dead first-swipe reports come from WebKit phones, so the
  // focus contract gets engine coverage there too.
  projects: [
    {
      name: "chromium",
      testIgnore: /webkit-mobile-scroll|mobile-sidebar/,
    },
    {
      name: "webkit",
      testMatch: /webkit-mobile-scroll/,
      use: { ...devices["Desktop WebKit"] },
    },
    {
      name: "mobile-chromium",
      testMatch: /mobile-sidebar/,
      use: { ...devices["Pixel 7"] },
    },
    {
      name: "mobile-webkit",
      testMatch: /mobile-sidebar/,
      use: { ...devices["iPhone 13"] },
    },
  ],
  webServer: {
    cwd: import.meta.dirname,
    command: [
      "npm run build",
      "cd ..",
      "mkdir -p build/e2e",
      "go build -tags webui -trimpath -buildvcs=false -o build/e2e/m2h .",
      `./build/e2e/m2h --no-open --host 127.0.0.1 --port ${port} web/e2e/docs`,
    ].join(" && "),
    url: `http://127.0.0.1:${port}`,
    timeout: 300_000,
    reuseExistingServer: !process.env.CI,
  },
});
