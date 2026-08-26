import { defineConfig } from "@playwright/test";

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
