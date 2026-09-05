import { expect, test } from "@playwright/test";

// Real-browser layout regressions for the reader shell. jsdom computes no
// geometry, so horizontal centering, the narrow-screen outline sheet, and the
// sidebar reveal of the active file can only be locked down with a genuine
// layout engine — the same reasoning as scroll-restoration.spec.ts.

// Wait until the document body has painted, so geometry assertions never race
// the client-rendered article.
async function waitForBody(
  page: import("@playwright/test").Page,
  path: string,
) {
  await page.goto(path);
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body p, .markdown-body h2") !== null,
  );
}

// Two animation frames: any scroll a wheel dispatch could have produced is
// applied before the next frame paints, so positions read afterwards reflect
// the settled state — without an arbitrary timeout.
async function waitForScrollSettle(page: import("@playwright/test").Page) {
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      }),
  );
}

// One geometry snapshot of the sidebar's single scroll container (the Base UI
// ScrollArea viewport), so isolation and normalization assertions all read
// the same source of truth.
async function readSidebarGeometry(page: import("@playwright/test").Page) {
  return page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (!(viewport instanceof HTMLElement)) {
      throw new Error("tree viewport was not rendered");
    }
    const rect = viewport.getBoundingClientRect();
    return {
      scrollTop: viewport.scrollTop,
      scrollLeft: viewport.scrollLeft,
      scrollHeight: viewport.scrollHeight,
      clientHeight: viewport.clientHeight,
      scrollWidth: viewport.scrollWidth,
      clientWidth: viewport.clientWidth,
      windowScrollY: window.scrollY,
      x: rect.x,
      y: rect.y,
      width: rect.width,
      height: rect.height,
    };
  });
}

async function setSidebarScrollTop(
  page: import("@playwright/test").Page,
  where: "top" | "max",
) {
  await page.evaluate((target) => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (viewport instanceof HTMLElement) {
      viewport.scrollTop =
        target === "top" ? 0 : viewport.scrollHeight - viewport.clientHeight;
    }
  }, where);
}

// Land on the nested fixture with the tree viewport already scrolled deep
// past both sticky ancestors — the reload + reveal flow from the tests below,
// factored out for the collapse regressions.
async function openRevealedNestedFile(page: import("@playwright/test").Page) {
  await page.setViewportSize({ width: 1280, height: 720 });
  await waitForBody(page, "/doc/a/b/c.md");

  const targetScrollY = await page.evaluate(() => {
    window.scrollTo(0, 220);
    return window.scrollY;
  });
  await page.waitForFunction(
    ([key, value]) => window.sessionStorage.getItem(key) === value,
    ["m2h.scroll.a/b/c.md", String(targetScrollY)],
  );

  await page.reload();
  await page.waitForFunction(
    () => document.querySelector('[aria-current="page"]') !== null,
  );
  // The reveal runs before first paint; wait until it has actually scrolled.
  await page.waitForFunction(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    return (
      viewport !== null && viewport !== undefined && viewport.scrollTop > 0
    );
  });
  return targetScrollY;
}

test("centers the capped document inside a wide canvas", async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 900 });
  await waitForBody(page, "/doc/scroll.md");

  const gaps = await page.evaluate(() => {
    const canvas = document.querySelector(".reader-canvas");
    const article = document.querySelector(".reader-document");
    if (canvas === null || article === null) {
      throw new Error("reader canvas or document was not rendered");
    }
    const canvasRect = canvas.getBoundingClientRect();
    const articleRect = article.getBoundingClientRect();
    return {
      canvasWidth: canvasRect.width,
      left: articleRect.left - canvasRect.left,
      right: canvasRect.right - articleRect.right,
    };
  });

  // 1600px minus the 256px sidebar leaves the canvas far wider than the
  // standard 980px cap, so the cap engages and the margins must match.
  expect(gaps.canvasWidth).toBeGreaterThan(1030);
  expect(Math.abs(gaps.left - gaps.right)).toBeLessThanOrEqual(1);
});

test("does not create page-level horizontal overflow in full width without TOC", async ({
  page,
}) => {
  // The fixture pairs a frontmatter panel (width:100% in its base rule) with
  // an H2 (so the TOC rail renders): full width + frontmatter + TOC hidden +
  // sidebar visible is the combination where a width contract that adds
  // margins on top of 100% spills past the canvas into a window-level
  // scrollbar.
  await page.setViewportSize({ width: 1440, height: 900 });
  await waitForBody(page, "/doc/layout-frontmatter.md");

  await page.getByRole("button", { name: /文档宽度：/ }).click();
  await page.getByRole("menuitemradio", { name: "全屏" }).click();

  await expect(page.locator(".reader-toc")).toBeVisible();

  await page.getByRole("button", { name: "隐藏文档目录" }).click();

  // The rail is no longer unmounted — the slot collapses to zero width (and
  // clips the sliding rail), which is what reclaims the canvas width.
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          document.querySelector(".reader-toc-slot")?.getBoundingClientRect()
            .width ?? -1,
      ),
    )
    .toBe(0);

  const geometry = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));

  expect(geometry.scrollWidth).toBeLessThanOrEqual(geometry.clientWidth);
});

test("collapses the width ladder on mobile viewports without touching the preference", async ({
  page,
}) => {
  // Below 768px the sidebar is off-canvas, so the reader owns the viewport.
  await page.setViewportSize({ width: 375, height: 720 });
  // A non-default width (`wide`): it survives the round trip untouched.
  await page.goto("/doc/layout-frontmatter.md?width=wide");
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body p, .markdown-body h2") !== null,
  );

  // The reader's configured width survives: pure CSS collapses the ladder,
  // never the URL or the stored preference.
  expect(await page.evaluate(() => window.location.search)).toBe("?width=wide");
  await expect(page.locator(".document-width-control")).toBeHidden();
  // The title's frontmatter summary hides; the full inspection panel stays.
  await expect(page.locator(".document-meta")).toBeHidden();
  await expect(page.locator(".reader-frontmatter")).toBeVisible();

  // The article spans the full viewport with no side gutter.
  const geometry = await page.evaluate(() => {
    const article = document.querySelector(".reader-document");
    if (article === null) {
      throw new Error("reader document was not rendered");
    }
    return {
      viewport: document.documentElement.clientWidth,
      left: article.getBoundingClientRect().left,
      width: article.getBoundingClientRect().width,
    };
  });
  expect(geometry.left).toBe(0);
  expect(geometry.width).toBe(geometry.viewport);

  // Widening the window restores the wide cap (and the menu) with the same
  // URL still in place.
  await page.setViewportSize({ width: 1440, height: 800 });
  await expect
    .poll(() =>
      page.evaluate(() => {
        const article = document.querySelector(".reader-document");
        return article === null ? -1 : article.getBoundingClientRect().width;
      }),
    )
    .toBeLessThanOrEqual(1280);
  expect(await page.evaluate(() => window.location.search)).toBe("?width=wide");
  await expect(page.locator(".document-width-control")).toBeVisible();
});

test("keeps the mobile sidebar tree scrollable on first open", async ({
  page,
}) => {
  // The mobile Sheet reuses the desktop sidebar shell, whose SidebarContent
  // carries an overflow-clip paint boundary for the desktop tree. Inside the
  // fixed modal that clip around the ScrollArea viewport left first-open
  // touch scrolling dead on mobile browsers until some interaction rebuilt
  // the scrolling layer — so mobile must drop the clip while desktop keeps
  // it, and the tree must overflow and scroll on the very first open.
  await page.setViewportSize({ width: 375, height: 720 });
  await waitForBody(page, "/doc/tree/note-24.md");

  await page.getByRole("button", { name: "切换文件导航" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  // The ScrollArea viewport stays the tree's only scroll container, it
  // really overflows, and the mobile SidebarContent clips nothing.
  const structure = await page.evaluate(() => {
    const content = document.querySelector('[data-slot="sidebar-content"]');
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (!(content instanceof HTMLElement)) {
      throw new Error("sidebar content was not rendered");
    }
    if (!(viewport instanceof HTMLElement)) {
      throw new Error("tree viewport was not rendered");
    }
    return {
      contentOverflow: getComputedStyle(content).overflow,
      viewportOverflowY: getComputedStyle(viewport).overflowY,
      overflowing: viewport.scrollHeight > viewport.clientHeight,
    };
  });
  expect(structure.contentOverflow).toBe("visible");
  expect(structure.viewportOverflowY).toBe("scroll");
  expect(structure.overflowing).toBe(true);

  // Touch scrolling works on this very first open, with no tree interaction
  // first: the active-file reveal has scrolled the viewport, so start it at
  // the top, then swipe up over the tree — the viewport moves, the reader
  // window behind the modal sheet does not.
  await page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (viewport instanceof HTMLElement) {
      viewport.scrollTop = 0;
    }
  });
  expect(await page.evaluate(() => window.scrollY)).toBe(0);

  const session = await page.context().newCDPSession(page);
  await session.send("Input.dispatchTouchEvent", {
    type: "touchStart",
    touchPoints: [{ x: 100, y: 500 }],
  });
  for (const y of [460, 420, 380, 340, 300]) {
    await session.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [{ x: 100, y }],
    });
  }
  await session.send("Input.dispatchTouchEvent", {
    type: "touchEnd",
    touchPoints: [],
  });

  await expect
    .poll(() =>
      page.evaluate(() => {
        const tree = document.querySelector('[aria-label="Markdown 文件树"]');
        const viewport = tree?.closest<HTMLElement>(
          '[data-slot="scroll-area-viewport"]',
        );
        return viewport === null || viewport === undefined
          ? -1
          : viewport.scrollTop;
      }),
    )
    .toBeGreaterThan(0);
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
});

test("opens the root README from the bare workspace address", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const documentRequests: string[] = [];
  page.on("request", (request) => {
    if (request.url().includes("/api/document")) {
      documentRequests.push(request.url());
    }
  });

  await page.goto("/");
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body p, .markdown-body h2") !== null,
  );

  // The workspace picks its own entry document: exactly one document fetch,
  // the address canonicalizes to the README's /doc/ URL (so refresh and share
  // land on the same reading state), and the tree marks it current.
  expect(documentRequests).toHaveLength(1);
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/README.md",
  );
  await expect(page.locator('[aria-current="page"]')).toContainText(
    "README.md",
  );

  // Hovering the entry in the sidebar reveals its tooltip: name, title, and
  // — when the document declares one — the frontmatter description wrapping
  // underneath.
  await page.locator('[aria-current="page"]').hover();
  await expect(page.locator(".tree-tooltip-name")).toHaveText("README.md");
  await expect(page.locator(".tree-tooltip-title")).toHaveText("工作区入口");
  await expect(page.locator(".tree-tooltip-description")).toContainText(
    "e2e 固定目录的入口文档",
  );

  // Picking a document from the tree still opens it under /doc/ (expanding
  // the collapsed directories on the way).
  await page.locator('[data-tree-path="a"]').click();
  await page.locator('[data-tree-path="a/b"]').click();
  await page.getByRole("button", { name: "笔记 A-01，a/b/a-01.md" }).click();
  await expect(page.locator(".markdown-body")).toBeVisible();
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/a/b/a-01.md",
  );
});

test("shows the image magnifier on hover and keyboard focus only", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await waitForBody(page, "/doc/images.md");
  await page.waitForFunction(
    () => document.querySelectorAll(".m2h-lightbox-trigger").length > 0,
  );

  const readOpacity = () =>
    page.evaluate(() => {
      const trigger = document.querySelector(
        ".m2h-lightbox-trigger",
      ) as HTMLElement | null;
      return trigger === null ? null : getComputedStyle(trigger).opacity;
    });

  // Idle: the trigger is fully faded out and click-transparent.
  await expect.poll(readOpacity).toBe("0");

  // Hovering the image's frame fades it in (the 120ms transition settles).
  await page.hover(".m2h-image-frame");
  await expect.poll(readOpacity).toBe("1");

  // Keyboard focus keeps it visible: :focus-visible must survive the hover
  // gating or Tab users would reach an invisible button. Park the mouse away
  // from the image so hover cannot do the work, then Tab to the trigger.
  await page.mouse.move(1, 1);
  await page.evaluate(() => {
    (document.activeElement as HTMLElement | null)?.blur();
  });
  for (let step = 0; step < 50; step += 1) {
    const focused = await page.evaluate(() =>
      document.activeElement?.classList.contains("m2h-lightbox-trigger"),
    );
    if (focused === true) {
      break;
    }
    await page.keyboard.press("Tab");
  }
  await expect
    .poll(() =>
      page.evaluate(() =>
        document.activeElement?.classList.contains("m2h-lightbox-trigger"),
      ),
    )
    .toBe(true);
  await expect.poll(readOpacity).toBe("1");
});

test("keeps the floating navigation beside the reader canvas, clear of the TOC rail", async ({
  page,
}) => {
  // scroll.md has one H2, so the TOC rail renders and offsets the pair.
  await page.setViewportSize({ width: 1440, height: 800 });
  await waitForBody(page, "/doc/scroll.md");
  await expect(page.locator(".reader-toc")).toBeVisible();

  // The navigation offset now animates with the rail (200ms linear), so the
  // geometry is polled until the pair settles fully left of the rail with
  // the standard 1.5rem gap.
  await expect
    .poll(() =>
      page.evaluate(() => {
        const nav = document.querySelector(".reader-navigation");
        const toc = document.querySelector(".reader-toc");
        if (nav === null || toc === null) {
          throw new Error("navigation or TOC rail was not rendered");
        }
        return Math.round(
          toc.getBoundingClientRect().left - nav.getBoundingClientRect().right,
        );
      }),
    )
    .toBe(24);

  // Below 1200px the rail hides and the pair returns to the viewport edge
  // once the offset transition settles.
  await page.setViewportSize({ width: 1100, height: 800 });
  await expect(page.locator(".reader-toc")).toBeHidden();
  await expect
    .poll(() =>
      page.evaluate(() => {
        const nav = document.querySelector(".reader-navigation");
        if (nav === null) {
          throw new Error("reader navigation was not rendered");
        }
        return Math.round(
          window.innerWidth - nav.getBoundingClientRect().right,
        );
      }),
    )
    .toBe(24);
});

test("fades the sidebar scrollbar in while scrolling and out after it stops", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  // The tree fixture holds 24 notes, so the sidebar viewport overflows.
  await waitForBody(page, "/doc/tree/note-24.md");

  const readScrollbar = () =>
    page.evaluate(() => {
      const root = document.querySelector(
        '.tree-scroll[data-scrollbar-visibility="scrolling"]',
      ) as HTMLElement | null;
      const bar = root?.querySelector<HTMLElement>(
        '[data-slot="scroll-area-scrollbar"]',
      );
      if (root === null || root === undefined) {
        return { mounted: false, opacity: null, scrolling: null };
      }
      return {
        mounted: bar !== null && bar !== undefined,
        opacity: bar ? getComputedStyle(bar).opacity : null,
        scrolling: root.dataset.m2hScrolling ?? null,
      };
    });

  // The initial active-file reveal scrolls the tree (note-24 sits at its
  // bottom), which legitimately shows the scrollbar; let that burst settle
  // before asserting the idle state.
  await expect.poll(async () => (await readScrollbar()).scrolling).toBeNull();

  // Idle: the (overflowing) scrollbar is mounted but fully faded out.
  const idle = await readScrollbar();
  expect(idle.mounted).toBe(true);
  expect(idle.opacity).toBe("0");

  // A real wheel over the tree fades it in. Start from the top of the tree —
  // the reveal above already scrolled it to the bottom, where a downward
  // wheel has nowhere to go and no scroll event would fire.
  await setSidebarScrollTop(page, "top");
  const viewport = await readSidebarGeometry(page);
  await page.mouse.move(
    viewport.x + viewport.width / 2,
    viewport.y + viewport.height / 2,
  );
  await page.mouse.wheel(0, 400);
  await expect.poll(async () => (await readScrollbar()).scrolling).toBe("true");
  await expect.poll(async () => (await readScrollbar()).opacity).toBe("1");

  // … and ~700ms after the last scroll it fades back out.
  await expect
    .poll(async () => (await readScrollbar()).opacity, { timeout: 5_000 })
    .toBe("0");
  expect((await readScrollbar()).scrolling).toBeNull();
});

test("offers the outline in a sheet on narrow viewports", async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await waitForBody(page, "/doc/scroll.md");

  // Below 1200px the rail and its persistent toggle disappear; the toolbar
  // sheet trigger takes over instead of leaving a "pressed but invisible" TOC.
  await expect(page.locator(".reader-toc")).toBeHidden();
  await expect(page.locator(".reader-toc-toggle")).toBeHidden();

  const trigger = page.getByRole("button", { name: "打开文档目录" });
  await expect(trigger).toBeVisible();
  await trigger.click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  const link = dialog.getByRole("link", { name: "目标章节" });
  await expect(link).toBeVisible();
  await link.click();

  // The sheet closes first and hands the body scroll to the next frame; the
  // fragment is then recorded through the shared URL funnel.
  await expect(dialog).toBeHidden();
  await expect
    .poll(() => page.evaluate(() => window.location.hash))
    .toBe(encodeURI("#目标章节"));

  const heading = page.locator(".markdown-body h2", { hasText: "目标章节" });
  const toolbarBottom = await page.evaluate(() => {
    const toolbar = document.querySelector(".reader-toolbar");
    if (toolbar === null) {
      throw new Error("reader toolbar was not rendered");
    }
    return toolbar.getBoundingClientRect().bottom;
  });
  // The heading lands just below the sticky toolbar, not underneath it.
  await expect
    .poll(() =>
      heading.evaluate((element) => element.getBoundingClientRect().top),
    )
    .toBeGreaterThanOrEqual(toolbarBottom - 1);
  await expect
    .poll(() =>
      heading.evaluate((element) => element.getBoundingClientRect().top),
    )
    .toBeLessThanOrEqual(toolbarBottom + 200);
});

test("reveals the active file in the tree after a reload without moving the reader", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  // The tree fixture holds 24 notes; note-24 sits far below the sidebar's
  // initial fold, so the reload must scroll the tree viewport to reveal it.
  await waitForBody(page, "/doc/tree/note-24.md");

  const targetScrollY = await page.evaluate(() => {
    window.scrollTo(0, 200);
    return window.scrollY;
  });
  await page.waitForFunction(
    ([key, value]) => window.sessionStorage.getItem(key) === value,
    ["m2h.scroll.tree/note-24.md", String(targetScrollY)],
  );

  await page.reload();
  await page.waitForFunction(
    () => document.querySelector('[aria-current="page"]') !== null,
  );

  // The saved reading offset comes back first …
  await expect
    .poll(() => page.evaluate(() => window.scrollY))
    .toBe(targetScrollY);

  // … and the tree reveal brings the active file inside the sidebar viewport.
  await expect
    .poll(() =>
      page.evaluate(() => {
        const tree = document.querySelector('[aria-label="Markdown 文件树"]');
        const viewport = tree?.closest<HTMLElement>(
          '[data-slot="scroll-area-viewport"]',
        );
        const active = document.querySelector<HTMLElement>(
          '[aria-current="page"]',
        );
        if (viewport === null || viewport === undefined || active === null) {
          return null;
        }
        const viewportRect = viewport.getBoundingClientRect();
        const activeRect = active.getBoundingClientRect();
        return (
          activeRect.top >= viewportRect.top - 1 &&
          activeRect.bottom <= viewportRect.bottom + 1
        );
      }),
    )
    .toBe(true);

  // The reveal scrolled the tree (not just left it at the top) …
  await expect
    .poll(() =>
      page.evaluate(() => {
        const tree = document.querySelector('[aria-label="Markdown 文件树"]');
        const viewport = tree?.closest<HTMLElement>(
          '[data-slot="scroll-area-viewport"]',
        );
        return viewport === null || viewport === undefined
          ? -1
          : viewport.scrollTop;
      }),
    )
    .toBeGreaterThan(0);

  // … and it never touched the reader's restored position.
  expect(await page.evaluate(() => window.scrollY)).toBe(targetScrollY);
});

test("pins ancestor directories above the revealed active file", async ({
  page,
}) => {
  // The fixture nests a/b/ with 24 notes and c.md last, so revealing c.md
  // scrolls the tree well past both ancestor directories.
  const targetScrollY = await openRevealedNestedFile(page);

  const geometry = await page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    const dirA = document.querySelector<HTMLElement>('[data-tree-path="a"]');
    const dirB = document.querySelector<HTMLElement>('[data-tree-path="a/b"]');
    const active = document.querySelector<HTMLElement>('[aria-current="page"]');
    if (
      viewport === undefined ||
      viewport === null ||
      dirA === null ||
      dirB === null ||
      active === null
    ) {
      throw new Error("tree viewport or sticky rows were not rendered");
    }
    const viewportTop = viewport.getBoundingClientRect().top;
    const a = dirA.getBoundingClientRect();
    const b = dirB.getBoundingClientRect();
    const c = active.getBoundingClientRect();
    return {
      aTop: a.top - viewportTop,
      aBottom: a.bottom - viewportTop,
      bTop: b.top - viewportTop,
      bBottom: b.bottom - viewportTop,
      cTop: c.top - viewportTop,
    };
  });

  // Both ancestors stick to the viewport top, stacking exactly one row apart …
  expect(Math.abs(geometry.aTop)).toBeLessThanOrEqual(1);
  expect(Math.abs(geometry.bTop - geometry.aBottom)).toBeLessThanOrEqual(1);
  // … and the revealed file sits clear of the sticky stack, not under it.
  expect(geometry.cTop).toBeGreaterThanOrEqual(geometry.bBottom - 1);

  // The sticky reveal is a tree-viewport-only scroll: the reader keeps the
  // offset restored from sessionStorage.
  expect(await page.evaluate(() => window.scrollY)).toBe(targetScrollY);
});

test("keeps sidebar wheel scrolling isolated from the reader window", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await waitForBody(page, "/doc/tree/note-24.md");

  const readerY = await page.evaluate(() => {
    window.scrollTo(0, 200);
    return window.scrollY;
  });

  // Start from the top of the tree so a downward wheel has room to scroll.
  await setSidebarScrollTop(page, "top");
  const viewport = await readSidebarGeometry(page);
  await page.mouse.move(
    viewport.x + viewport.width / 2,
    viewport.y + viewport.height / 2,
  );

  // A wheel over the tree scrolls the tree viewport …
  await page.mouse.wheel(0, 500);
  await expect
    .poll(async () => (await readSidebarGeometry(page)).scrollTop)
    .toBeGreaterThan(0);
  // … and never the reader window behind it.
  expect(await page.evaluate(() => window.scrollY)).toBe(readerY);

  // Pinned at the bottom edge, further wheels must not scroll-chain into the
  // document — this is what overscroll-behavior-y: contain is for.
  await setSidebarScrollTop(page, "max");
  await page.mouse.wheel(0, 1000);
  await waitForScrollSettle(page);
  expect(await page.evaluate(() => window.scrollY)).toBe(readerY);

  // The same holds at the top edge, wheeling up and out of the tree.
  await setSidebarScrollTop(page, "top");
  await page.mouse.wheel(0, -1000);
  await waitForScrollSettle(page);
  expect(await page.evaluate(() => window.scrollY)).toBe(readerY);
});

test("normalizes the tree viewport after collapsing a sticky ancestor", async ({
  page,
}) => {
  const targetScrollY = await openRevealedNestedFile(page);

  // Collapsing the outermost sticky directory unmounts its whole subtree —
  // including the active file — in a single commit while the viewport is
  // scrolled deep into it.
  await page.locator('[data-tree-path="a"]').click();
  await expect(page.locator('[data-tree-path="a/b"]')).toBeHidden();

  const geometry = await readSidebarGeometry(page);
  // The viewport is re-clamped to the shrunken tree before paint …
  expect(geometry.scrollTop).toBeLessThanOrEqual(
    geometry.scrollHeight - geometry.clientHeight + 1,
  );
  // … keeps no lateral drift and never overflows horizontally at all.
  expect(geometry.scrollLeft).toBe(0);
  expect(geometry.scrollWidth).toBeLessThanOrEqual(geometry.clientWidth + 1);

  // The reader keeps the offset restored from sessionStorage.
  expect(await page.evaluate(() => window.scrollY)).toBe(targetScrollY);
});

test("reveals the active file again after re-expanding its collapsed ancestor", async ({
  page,
}) => {
  await openRevealedNestedFile(page);

  await page.locator('[data-tree-path="a"]').click();
  await expect(page.locator('[data-tree-path="a/b"]')).toBeHidden();

  // Re-expanding remounts the subtree with c.md as its last row, far below
  // the fold. The collapse reset the reveal's record, so it must run again
  // and bring the active file back inside the viewport.
  await page.locator('[data-tree-path="a"]').click();
  await expect(page.locator('[aria-current="page"]')).toBeVisible();

  await expect
    .poll(() =>
      page.evaluate(() => {
        const tree = document.querySelector('[aria-label="Markdown 文件树"]');
        const viewport = tree?.closest<HTMLElement>(
          '[data-slot="scroll-area-viewport"]',
        );
        const active = document.querySelector<HTMLElement>(
          '[aria-current="page"]',
        );
        if (viewport === null || viewport === undefined || active === null) {
          return null;
        }
        const viewportRect = viewport.getBoundingClientRect();
        const activeRect = active.getBoundingClientRect();
        return (
          activeRect.top >= viewportRect.top - 1 &&
          activeRect.bottom <= viewportRect.bottom + 1
        );
      }),
    )
    .toBe(true);
});

test("jumps between document edges with the floating navigation", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await waitForBody(page, "/doc/scroll.md");

  const up = page.getByRole("button", { name: "回到顶部" });
  const down = page.getByRole("button", { name: "前往底部" });

  // At the very top the upward jump reports "already there" by disabling —
  // the affordance stays visible instead of hiding on a timer.
  await expect(up).toBeDisabled();
  await expect(down).toBeEnabled();

  // The downward jump smooth-scrolls the window to the document's bottom edge.
  await down.click();
  await expect
    .poll(() =>
      page.evaluate(() => {
        const max = document.documentElement.scrollHeight - window.innerHeight;
        return Math.abs(window.scrollY - max);
      }),
    )
    .toBeLessThanOrEqual(2);
  await expect(down).toBeDisabled();
  await expect(up).toBeEnabled();

  // … and the upward jump returns to the top, where the states swap back.
  await up.click();
  await expect
    .poll(() => page.evaluate(() => window.scrollY))
    .toBeLessThanOrEqual(1);
  await expect(up).toBeDisabled();
  await expect(down).toBeEnabled();

  // The jump pair floats fully inside the viewport, clear of its bottom edge.
  const geometry = await page.evaluate(() => {
    const nav = document.querySelector(".reader-navigation");
    if (nav === null) {
      throw new Error("reader navigation was not rendered");
    }
    const rect = nav.getBoundingClientRect();
    return { bottom: rect.bottom, innerHeight: window.innerHeight };
  });
  expect(geometry.bottom).toBeLessThanOrEqual(geometry.innerHeight);
});
