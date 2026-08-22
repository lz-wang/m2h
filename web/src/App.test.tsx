import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { APIError, type FileListResponse, type PreviewAPI } from "./api";
import { readScrollPosition, saveScrollPosition } from "./lib/scroll-position";

const initialFiles: FileListResponse = {
  kind: "directory",
  version: "0.9.1",
  roots: [
    {
      id: "r0",
      name: "docs",
      kind: "directory",
      absolutePath: "/tmp/docs",
      pathSeparator: "/",
      files: [
        { path: "README.md", name: "README.md", title: "Readme API Title" },
        { path: "guides/setup.md", name: "setup.md", title: "Setup API Title" },
      ],
    },
  ],
  defaultDocument: { root: "r0", path: "README.md" },
};

beforeEach(() => {
  window.localStorage.clear();
  window.sessionStorage.clear();
  window.history.replaceState(null, "", "/");
  document.documentElement.className = "";
  delete document.documentElement.dataset.mode;
  // index.html loads the Markdown stylesheet once as a stable /ui/markdown.css;
  // mirror that here so tests can assert applyTheme never swaps its href.
  document.getElementById("m2h-markdown-styles")?.remove();
  const stylesheet = document.createElement("link");
  stylesheet.id = "m2h-markdown-styles";
  stylesheet.rel = "stylesheet";
  stylesheet.href = "/ui/markdown.css";
  document.head.append(stylesheet);
});

describe("App directory preview", () => {
  it("selects the server default and uses API title metadata", async () => {
    const api = createAPI();
    render(<App api={api} />);

    await screen.findByText("Body for README.md");
    expect(api.listFiles).toHaveBeenCalledTimes(1);
    expect(api.getDocument).toHaveBeenCalledWith(
      "README.md",
      expect.any(AbortSignal),
    );
    expect(window.location.pathname + window.location.search).toBe(
      "/doc/README.md",
    );
    expect(document.title).toBe("Readme API Title");
    expect(document.documentElement.dataset.mode).toBe("auto");
    expect(
      document.getElementById("m2h-markdown-styles")?.getAttribute("href"),
    ).toBe("/ui/markdown.css");
    expect(screen.getByText("2 个 Markdown 文件")).toBeTruthy();
    const title = screen.getByRole("region", { name: "当前文档标题" });
    expect(title.textContent).toBe("Readme API Title");
    expect(
      screen.getByRole("button", { name: "显示主题：跟随系统" }),
    ).toBeTruthy();
  });

  it("opens the project and release links from the sidebar footer in new tabs", async () => {
    const view = render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const repository = screen.getByRole("link", {
      name: "在新页面打开 m2h GitHub 仓库",
    });
    expect(repository.getAttribute("href")).toBe(
      "https://github.com/lz-wang/m2h",
    );
    expect(repository.getAttribute("target")).toBe("_blank");
    expect(repository.getAttribute("rel")).toContain("noreferrer");

    const release = screen.getByRole("link", {
      name: "在新页面打开 m2h v0.9.1 发布信息",
    });
    const footer = repository.closest('[data-slot="sidebar-footer"]');
    expect(footer?.classList).toContain("justify-start");
    expect(footer?.classList).toContain("gap-1");
    expect(footer?.classList).not.toContain("justify-between");
    expect(repository.nextElementSibling).toBe(release);
    expect(release.getAttribute("href")).toBe(
      "https://github.com/lz-wang/m2h/releases/tag/v0.9.1",
    );
    expect(release.getAttribute("target")).toBe("_blank");
    view.unmount();

    render(
      <App
        api={createAPI({
          listFiles: vi.fn().mockResolvedValue({
            ...initialFiles,
            version: "dev-20260812-abcdef0",
          }),
        })}
      />,
    );
    const development = await screen.findByRole("link", {
      name: "在新页面打开 m2h dev-20260812-abcdef0 发布信息",
    });
    expect(development.getAttribute("href")).toBe(
      "https://github.com/lz-wang/m2h/releases",
    );
    expect(development.getAttribute("target")).toBe("_blank");
  });

  it("hot-swaps the document body on a server-sent document-changed event", async () => {
    const getDocument = vi
      .fn<PreviewAPI["getDocument"]>()
      .mockResolvedValueOnce({
        path: "README.md",
        title: "Readme API Title",
        html: "<p>Original body</p>",
        frontmatter: null,
        toc: [],
      })
      .mockResolvedValueOnce({
        path: "README.md",
        title: "Readme API Title",
        html: "<p>Updated body</p>",
        frontmatter: null,
        toc: [],
      });
    const api = createAPI({ getDocument });
    const events = stubEventSource();
    render(<App api={api} />);

    await screen.findByText("Original body");
    expect(getDocument).toHaveBeenCalledTimes(1);

    await act(async () => {
      events.dispatch("document-changed");
    });
    await screen.findByText("Updated body");
    expect(getDocument).toHaveBeenCalledTimes(2);
  });

  it("hides file navigation for a single-file preview", async () => {
    const api = createAPI({
      listFiles: vi.fn().mockResolvedValue({
        kind: "single",
        version: "0.9.1",
        roots: [
          {
            id: "r0",
            name: "README.md",
            kind: "file",
            absolutePath: "/tmp/README.md",
            pathSeparator: "/",
            files: [
              {
                path: "README.md",
                name: "README.md",
                title: "Readme API Title",
              },
            ],
          },
        ],
        defaultDocument: { root: "r0", path: "README.md" },
      }),
    });
    render(<App api={api} />);

    await screen.findByText("Body for README.md");
    expect(screen.queryByRole("button", { name: "切换文件导航" })).toBeNull();
    expect(screen.queryByRole("searchbox", { name: "搜索文档" })).toBeNull();
    // Shared toolbar controls remain available in single-file mode.
    expect(screen.getByRole("button", { name: "文档宽度：标准" })).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "显示主题：跟随系统" }),
    ).toBeTruthy();
  });

  it("groups a multi-root workspace into labeled, expanded root trees", async () => {
    const getDocument = vi.fn().mockImplementation(async (path: string) => ({
      path,
      title: path.includes("r1/") ? "Beta Readme" : "Alpha Readme",
      html: `<h1>${path.includes("r1/") ? "Beta" : "Alpha"}</h1>`,
      frontmatter: null,
      toc: [],
    }));
    const api = createAPI({
      listFiles: vi.fn().mockResolvedValue({
        kind: "workspace",
        version: "0.9.1",
        roots: [
          {
            id: "r0",
            name: "alpha",
            kind: "directory",
            absolutePath: "/tmp/alpha",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Alpha Readme" },
            ],
          },
          {
            id: "r1",
            name: "beta",
            kind: "directory",
            absolutePath: "/tmp/beta",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Beta Readme" },
              {
                path: "guide/intro.md",
                name: "intro.md",
                title: "Intro",
              },
            ],
          },
        ],
        defaultDocument: { root: "r0", path: "README.md" },
      }),
      getDocument,
    });
    render(<App api={api} />);

    // The workspace default opens the primary root's document under its
    // virtual key and the URL carries the key unchanged.
    await screen.findByRole("heading", { level: 1, name: "Alpha" });
    expect(getDocument).toHaveBeenCalledWith(
      "r0/README.md",
      expect.any(AbortSignal),
    );
    expect(window.location.pathname).toBe("/doc/r0/README.md");

    // Both roots render as labeled top-level rows, expanded by default so the
    // workspace is immediately browsable.
    for (const label of ["alpha", "beta"]) {
      const row = screen.getByRole("button", { name: label });
      expect(row.getAttribute("aria-expanded")).toBe("true");
    }
    expect(screen.getByText("3 个 Markdown 文件")).toBeTruthy();

    // Same-named documents in two roots stay distinct; the second root's copy
    // is selectable through its own virtual key while showing its plain
    // root-relative name.
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Beta Readme，r1/README.md" }),
    );
    await screen.findByRole("heading", { level: 1, name: "Beta" });
    expect(getDocument).toHaveBeenLastCalledWith(
      "r1/README.md",
      expect.any(AbortSignal),
    );
    expect(window.location.pathname).toBe("/doc/r1/README.md");

    // Search stays global across roots but keeps the root grouping: matching
    // a root's name surfaces every document under it.
    const search = screen.getByRole("searchbox", { name: "搜索文档" });
    await user.clear(search);
    await user.type(search, "beta");
    expect(
      screen.getByRole("button", { name: "Beta Readme，r1/README.md" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Intro，r1/guide/intro.md" }),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Alpha Readme，r0/README.md" }),
    ).toBeNull();
    expect(screen.getByText("2 个 Markdown 文件")).toBeTruthy();

    // A query matching neither root nor document leaves the empty state.
    await user.clear(search);
    await user.type(search, "missing");
    expect(screen.getByText("没有匹配的文档")).toBeTruthy();
  });

  it("restores a dark deep link and expands the selected directory", async () => {
    window.history.replaceState(
      null,
      "",
      "/doc/guides/setup.md?mode=dark#install",
    );
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "guides/setup.md",
        title: "Setup API Title",
        html: '<h2 id="install">Install</h2>',
      }),
    });
    render(<App api={api} />);

    await screen.findByRole("heading", { level: 2, name: "Install" });
    expect(api.getDocument).toHaveBeenCalledWith(
      "guides/setup.md",
      expect.any(AbortSignal),
    );
    expect(
      screen
        .getByRole("button", { name: "guides" })
        .getAttribute("aria-expanded"),
    ).toBe("true");
    expect(
      screen
        .getByRole("button", { name: "Setup API Title，guides/setup.md" })
        .getAttribute("aria-current"),
    ).toBe("page");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(
      document.getElementById("m2h-markdown-styles")?.getAttribute("href"),
    ).toBe("/ui/markdown.css");
    expect(window.location.hash).toBe("#install");
    await waitFor(() =>
      expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith(
        expect.objectContaining({ block: "start" }),
      ),
    );
  });

  it("keeps deeply nested tree items aligned to the root trailing edge", async () => {
    const path = "internal/markdown/testdata/gfm.md";
    const title = "GFM Fixture";
    const file = { path, name: "gfm.md", title };
    const api = createAPI({
      listFiles: vi.fn().mockResolvedValue({
        kind: "directory",
        version: "0.9.1",
        roots: [
          {
            id: "r0",
            name: "testdata",
            absolutePath: "/tmp/testdata",
            pathSeparator: "/",
            files: [file],
          },
        ],
        defaultDocument: { root: "r0", path },
      }),
      getDocument: vi.fn().mockResolvedValue({
        path,
        title,
        html: "<p>Nested fixture</p>",
      }),
    });
    render(<App api={api} />);

    const fileButton = await screen.findByRole("button", {
      name: `${title}，${path}`,
    });
    const submenus: HTMLElement[] = [];
    for (
      let ancestor = fileButton.parentElement;
      ancestor !== null;
      ancestor = ancestor.parentElement
    ) {
      if (
        ancestor instanceof HTMLElement &&
        ancestor.dataset.sidebar === "menu-sub"
      ) {
        submenus.push(ancestor);
      }
    }

    expect(submenus).toHaveLength(3);
    for (const submenu of submenus) {
      expect(submenu.classList).toContain("ml-3.5");
      expect(submenu.classList).toContain("pl-2.5");
      expect(submenu.classList).not.toContain("mx-3.5");
      expect(submenu.classList).not.toContain("px-2.5");
    }
  });

  it("preserves cross-document fragments for keyboard navigation and theme changes", async () => {
    const user = userEvent.setup();
    const api = createAPI({
      getDocument: vi.fn().mockImplementation(async (path: string) => {
        if (path === "README.md") {
          return {
            path,
            title: "Readme API Title",
            html: '<a href="/doc/guides/setup.md#install">Setup section</a>',
          };
        }
        return {
          path,
          title: "Setup API Title",
          html: '<h2 id="install">Install</h2>',
        };
      }),
    });
    render(<App api={api} />);
    const link = await screen.findByRole("link", { name: "Setup section" });

    fireEvent.keyDown(link, { key: "Enter" });
    await screen.findByRole("heading", { level: 2, name: "Install" });
    await settleScrollPosition();
    expect(
      window.location.pathname + window.location.search + window.location.hash,
    ).toBe("/doc/guides/setup.md#install");

    await user.click(
      screen.getByRole("button", { name: "显示主题：跟随系统" }),
    );
    await user.click(
      await screen.findByRole("menuitemradio", { name: "深色" }),
    );
    expect(window.location.search + window.location.hash).toBe(
      "?mode=dark#install",
    );
  });

  it("pushes document selections and restores popstate routes", async () => {
    const user = userEvent.setup();
    const api = createAPI();
    render(<App api={api} />);
    await screen.findByText("Body for README.md");

    await user.click(screen.getByRole("button", { name: "guides" }));
    await user.click(
      screen.getByRole("button", { name: "Setup API Title，guides/setup.md" }),
    );
    await screen.findByText("Body for guides/setup.md");
    expect(window.location.pathname + window.location.search).toBe(
      "/doc/guides/setup.md",
    );

    window.history.replaceState(null, "", "/doc/README.md?mode=light");
    window.dispatchEvent(new PopStateEvent("popstate"));
    await screen.findByText("Body for README.md");
    expect(screen.getByRole("button", { name: "显示主题：浅色" })).toBeTruthy();
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("does not expose a manual directory refresh control", async () => {
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    expect(screen.queryByRole("button", { name: "刷新文件列表" })).toBeNull();
  });

  it("supports keyboard tree toggles and the theme menu", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const folder = screen.getByRole("button", { name: "guides" });
    folder.focus();
    await user.keyboard("{Enter}");
    expect(folder.getAttribute("aria-expanded")).toBe("true");

    await user.click(
      screen.getByRole("button", { name: "显示主题：跟随系统" }),
    );
    await user.click(
      await screen.findByRole("menuitemradio", { name: "深色" }),
    );
    expect(screen.getByRole("button", { name: "显示主题：深色" })).toBeTruthy();
    expect(window.location.search).toBe("?mode=dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("switches document width presets and uses matching toolbar icons", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    expect(screen.getByRole("button", { name: "文档宽度：标准" })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "文档宽度：标准" }));
    await user.click(
      await screen.findByRole("menuitemradio", { name: "全屏" }),
    );
    expect(screen.getByRole("button", { name: "文档宽度：全屏" })).toBeTruthy();
    expect(document.querySelector(".reader-canvas-full")).toBeTruthy();
    expect(window.location.search).toBe("?width=full");

    expect(
      screen
        .getByRole("button", { name: "显示主题：跟随系统" })
        .querySelector(".lucide-sun-moon"),
    ).toBeTruthy();
    await user.click(
      screen.getByRole("button", { name: "显示主题：跟随系统" }),
    );
    await user.click(
      await screen.findByRole("menuitemradio", { name: "浅色" }),
    );
    expect(
      screen
        .getByRole("button", { name: "显示主题：浅色" })
        .querySelector(".lucide-sun"),
    ).toBeTruthy();
  });

  it("shows consistent tooltips for non-TOC toolbar actions except the title", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const targets = [
      [screen.getByRole("button", { name: "切换文件导航" }), "收起文件导航"],
      [screen.getByRole("button", { name: "文档宽度：标准" }), "调整文档宽度"],
      [
        screen.getByRole("button", { name: "显示主题：跟随系统" }),
        "切换显示主题",
      ],
    ] as const;

    for (const [target, content] of targets) {
      await user.hover(target);
      expect(
        await screen.findByText(content, {
          selector: '[data-slot="tooltip-content"]',
        }),
      ).toBeTruthy();
      await user.unhover(target);
    }

    await user.hover(screen.getByRole("region", { name: "当前文档标题" }));
    expect(
      screen.queryByText("README.md", { selector: '[role="tooltip"]' }),
    ).toBeNull();
  });

  it("places the TOC control at the right edge of the toolbar", async () => {
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const tocToggle = screen.getByRole("button", {
      name: "当前文档没有目录",
    });
    const themeToggle = screen.getByRole("button", {
      name: "显示主题：跟随系统",
    });
    expect(
      tocToggle.compareDocumentPosition(themeToggle) &
        Node.DOCUMENT_POSITION_PRECEDING,
    ).toBe(Node.DOCUMENT_POSITION_PRECEDING);
  });

  it("shares the open document from the toolbar with clean links, local path, and raw source", async () => {
    const user = userEvent.setup();
    window.history.replaceState(
      null,
      "",
      "/doc/README.md?mode=dark&width=wide#install",
    );
    // jsdom has no clipboard and no execCommand: capture the fallback path's
    // textarea content so every copied value is asserted verbatim.
    const copied: string[] = [];
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: () => {
        copied.push(document.querySelector("textarea")?.value ?? "");
        return true;
      },
    });
    const api = createAPI();
    try {
      render(<App api={api} />);
      await screen.findByText("Body for README.md");

      // The share trigger sits left of the width menu.
      const share = screen.getByRole("button", { name: "分享文档" });
      const width = screen.getByRole("button", { name: "文档宽度：宽" });
      expect(
        share.compareDocumentPosition(width) & Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);

      const openMenu = async () => {
        await user.click(screen.getByRole("button", { name: "分享文档" }));
        await screen.findByRole("menuitem", { name: "复制文档网页链接" });
      };
      const copyFromMenu = async (name: string) => {
        await openMenu();
        await user.click(screen.getByRole("menuitem", { name }));
      };

      // Document page link: the current heading hash is kept, the sender's
      // mode/width preferences are not.
      await copyFromMenu("复制文档网页链接");
      await waitFor(() =>
        expect(copied.at(-1)).toBe("http://localhost/doc/README.md#install"),
      );
      expect((await screen.findByRole("status")).textContent).toBe(
        "已复制文档链接",
      );

      await copyFromMenu("复制文档本地路径");
      await waitFor(() => expect(copied.at(-1)).toBe("/tmp/docs/README.md"));
      expect((await screen.findByRole("status")).textContent).toBe(
        "已复制本地路径",
      );

      await copyFromMenu("复制 Markdown 网页链接");
      await waitFor(() =>
        expect(copied.at(-1)).toBe("http://localhost/raw/README.md"),
      );
      expect((await screen.findByRole("status")).textContent).toBe(
        "已复制 Markdown 链接",
      );

      // Full text is fetched lazily from /raw/ only at click time.
      expect(api.getMarkdown).not.toHaveBeenCalled();
      await copyFromMenu("复制 Markdown 全文");
      expect(api.getMarkdown).toHaveBeenCalledWith("README.md");
      await waitFor(() =>
        expect(copied.at(-1)).toBe("# Raw source of README.md\n"),
      );
      expect((await screen.findByRole("status")).textContent).toBe(
        "已复制 Markdown",
      );
    } finally {
      Reflect.deleteProperty(document, "execCommand");
    }
  });

  it("filters documents locally by title and file name", async () => {
    const user = userEvent.setup();
    const api = createAPI();
    render(<App api={api} />);
    await screen.findByText("Body for README.md");

    const search = screen.getByRole("searchbox", { name: "搜索文档" });
    await user.type(search, "setup api");
    expect(
      screen.getByRole("button", {
        name: "Setup API Title，guides/setup.md",
      }),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Readme API Title，README.md" }),
    ).toBeNull();
    expect(screen.getByText("1 个 Markdown 文件")).toBeTruthy();

    await user.clear(search);
    await user.type(search, "missing.md");
    expect(screen.getByText("没有匹配的文档")).toBeTruthy();
    expect(api.listFiles).toHaveBeenCalledTimes(1);
    expect(api.getDocument).toHaveBeenCalledTimes(1);
  });

  it("exposes full file names and resizes the desktop sidebar without drag transitions", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const file = screen.getByRole("button", {
      name: "Readme API Title，README.md",
    });
    await user.hover(file);
    const tooltipName = await screen.findByText("README.md", {
      selector: ".tree-tooltip-name",
    });
    expect(
      tooltipName.closest('[data-slot="tooltip-content"]')?.textContent,
    ).toContain("Readme API Title");
    const resize = screen.getByRole("button", { name: "调整侧边栏宽度" });
    const sidebar = document.querySelector<HTMLElement>(
      '[data-slot="sidebar"]',
    );
    const gap = document.querySelector<HTMLElement>(
      '[data-slot="sidebar-gap"]',
    );
    const container = document.querySelector<HTMLElement>(
      '[data-slot="sidebar-container"]',
    );
    if (sidebar === null || gap === null || container === null) {
      throw new Error("desktop sidebar layout was not rendered");
    }

    expect(gap.classList).toContain("transition-[width]");
    expect(container.classList).toContain("transition-[left,right,width]");
    fireEvent.pointerDown(resize, { clientX: 256 });
    expect(sidebar.dataset.resizing).toBe("true");
    expect(gap.classList).toContain("transition-none");
    expect(container.classList).toContain("transition-none");
    fireEvent.pointerMove(window, { clientX: 356 });
    expect(
      document
        .querySelector('[data-slot="sidebar-wrapper"]')
        ?.getAttribute("style"),
    ).toContain("356px");
    expect(
      JSON.parse(window.localStorage.getItem("m2h.preview.layout") ?? "{}"),
    ).toMatchObject({
      sidebarWidth: 256,
    });
    fireEvent.pointerUp(window);
    expect(sidebar.dataset.resizing).toBeUndefined();
    expect(gap.classList).toContain("transition-[width]");
    expect(container.classList).toContain("transition-[left,right,width]");
    expect(
      document
        .querySelector('[data-slot="sidebar-wrapper"]')
        ?.getAttribute("style"),
    ).toContain("356px");
    expect(
      JSON.parse(window.localStorage.getItem("m2h.preview.layout") ?? "{}"),
    ).toMatchObject({
      sidebarWidth: 356,
    });
  });

  it("opens a four-action context menu on file rows without changing the selection", async () => {
    const user = userEvent.setup();
    const copied: string[] = [];
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: () => {
        copied.push(document.querySelector("textarea")?.value ?? "");
        return true;
      },
    });
    const api = createAPI();
    try {
      render(<App api={api} />);
      await screen.findByText("Body for README.md");
      await user.click(screen.getByRole("button", { name: "guides" }));

      // Right-click a file row that is not the open document.
      fireEvent.contextMenu(
        screen.getByRole("button", {
          name: "Setup API Title，guides/setup.md",
        }),
      );

      // "新页面打开" keeps the browser's native link semantics.
      const open = await screen.findByRole("menuitem", { name: "新页面打开" });
      expect(open.getAttribute("href")).toBe("/doc/guides/setup.md");
      expect(open.getAttribute("target")).toBe("_blank");
      expect(open.getAttribute("rel")).toContain("noopener");
      expect(open.getAttribute("rel")).toContain("noreferrer");
      for (const name of [
        "复制文档网页链接",
        "复制文档本地路径",
        "复制 Markdown 网页链接",
      ]) {
        expect(screen.getByRole("menuitem", { name })).toBeTruthy();
      }

      // Right-clicking navigated nowhere: the open document, its body and
      // its request count are all unchanged.
      expect(screen.getByText("Body for README.md")).toBeTruthy();
      expect(window.location.pathname).toBe("/doc/README.md");
      expect(api.getDocument).toHaveBeenCalledTimes(1);

      await user.click(
        screen.getByRole("menuitem", { name: "复制文档网页链接" }),
      );
      await waitFor(() =>
        expect(copied.at(-1)).toBe("http://localhost/doc/guides/setup.md"),
      );
      expect((await screen.findByRole("status")).textContent).toBe(
        "已复制文档链接",
      );
      expect(api.getDocument).toHaveBeenCalledTimes(1);
    } finally {
      Reflect.deleteProperty(document, "execCommand");
    }
  });

  it("opens a folder-only context menu on directory rows", async () => {
    const user = userEvent.setup();
    const copied: string[] = [];
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: () => {
        copied.push(document.querySelector("textarea")?.value ?? "");
        return true;
      },
    });
    try {
      render(<App api={createAPI()} />);
      await screen.findByText("Body for README.md");
      // Captured before the menu opens: an open Base UI menu hides the rest
      // of the page from the accessibility tree.
      const guidesRow = screen.getByRole("button", { name: "guides" });
      expect(guidesRow.getAttribute("aria-expanded")).toBe("false");

      fireEvent.contextMenu(guidesRow);
      const copyItem = await screen.findByRole("menuitem", {
        name: "复制文件夹本地路径",
      });

      // The folder menu carries no document actions.
      expect(screen.queryByRole("menuitem", { name: "新页面打开" })).toBeNull();
      expect(
        screen.queryByRole("menuitem", { name: "复制文档网页链接" }),
      ).toBeNull();

      await user.click(copyItem);
      await waitFor(() => expect(copied.at(-1)).toBe("/tmp/docs/guides"));
      expect((await screen.findByRole("status")).textContent).toBe(
        "已复制文件夹路径",
      );

      // Right-click neither expanded the directory nor selected anything:
      // the collapsed child row stays absent and the open document is
      // untouched.
      expect(
        screen.queryByRole("button", {
          name: "Setup API Title，guides/setup.md",
        }),
      ).toBeNull();
      expect(guidesRow.getAttribute("aria-expanded")).toBe("false");
      expect(screen.getByText("Body for README.md")).toBeTruthy();
      expect(window.location.pathname).toBe("/doc/README.md");
    } finally {
      Reflect.deleteProperty(document, "execCommand");
    }
  });

  it("opens root-row folder menus and root-prefixed file menus in a workspace", async () => {
    const user = userEvent.setup();
    const copied: string[] = [];
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: () => {
        copied.push(document.querySelector("textarea")?.value ?? "");
        return true;
      },
    });
    const api = createAPI({
      listFiles: vi.fn().mockResolvedValue({
        kind: "workspace",
        version: "0.9.1",
        roots: [
          {
            id: "r0",
            name: "alpha",
            kind: "directory",
            absolutePath: "/tmp/alpha",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Alpha Readme" },
            ],
          },
          {
            id: "r1",
            name: "beta",
            kind: "directory",
            absolutePath: "/tmp/beta",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Beta Readme" },
            ],
          },
        ],
        defaultDocument: { root: "r0", path: "README.md" },
      }),
      getDocument: vi.fn().mockImplementation(async (path: string) => ({
        path,
        title: path.startsWith("r1/") ? "Beta Readme" : "Alpha Readme",
        html: `<p>Body for ${path}</p>`,
        frontmatter: null,
        toc: [],
      })),
    });
    try {
      render(<App api={api} />);
      await screen.findByText("Body for r0/README.md");

      // The root row is a folder: its menu copies the root's own path.
      fireEvent.contextMenu(screen.getByRole("button", { name: "beta" }));
      await user.click(
        await screen.findByRole("menuitem", { name: "复制文件夹本地路径" }),
      );
      await waitFor(() => expect(copied.at(-1)).toBe("/tmp/beta"));

      // Same-named files carry their own root's addresses, never the other
      // root's.
      fireEvent.contextMenu(
        screen.getByRole("button", { name: "Alpha Readme，r0/README.md" }),
      );
      expect(
        (
          await screen.findByRole("menuitem", { name: "新页面打开" })
        ).getAttribute("href"),
      ).toBe("/doc/r0/README.md");
      await user.click(
        screen.getByRole("menuitem", { name: "复制 Markdown 网页链接" }),
      );
      await waitFor(() =>
        expect(copied.at(-1)).toBe("http://localhost/raw/r0/README.md"),
      );

      fireEvent.contextMenu(
        screen.getByRole("button", { name: "Beta Readme，r1/README.md" }),
      );
      expect(
        (
          await screen.findByRole("menuitem", { name: "新页面打开" })
        ).getAttribute("href"),
      ).toBe("/doc/r1/README.md");
      await user.click(
        screen.getByRole("menuitem", { name: "复制文档本地路径" }),
      );
      await waitFor(() => expect(copied.at(-1)).toBe("/tmp/beta/README.md"));
      // The right-clicked second-root document never became the open one.
      expect(window.location.pathname).toBe("/doc/r0/README.md");
    } finally {
      Reflect.deleteProperty(document, "execCommand");
    }
  });

  it("reveals absolute directory paths through hover tooltips", async () => {
    const user = userEvent.setup();

    // Single-root preview: hovering the guides directory joins the server's
    // absolute root path with the root-relative tree path.
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    await user.hover(screen.getByRole("button", { name: "guides" }));
    const directoryPath = await screen.findByText("/tmp/docs/guides", {
      selector: ".tree-tooltip-path",
    });
    expect(
      directoryPath.closest('[data-slot="tooltip-content"]')?.classList,
    ).toContain("tree-tooltip-wide");
    // The native title tooltip is gone; the accessible name stays the label.
    const guidesRow = screen.getByRole("button", { name: "guides" });
    expect(guidesRow.getAttribute("title")).toBeNull();
    await user.unhover(guidesRow);

    // Multi-root workspace: the labeled root row shows its own path and a
    // nested directory composes root + relative segments with the
    // server-reported separator (here a Windows-style one).
    const workspaceAPI = createAPI({
      listFiles: vi.fn().mockResolvedValue({
        kind: "workspace",
        version: "0.9.1",
        roots: [
          {
            id: "r0",
            name: "alpha",
            kind: "directory",
            absolutePath: "/tmp/alpha",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Alpha Readme" },
            ],
          },
          {
            id: "r1",
            name: "beta",
            kind: "directory",
            absolutePath: "D:\\work\\beta",
            pathSeparator: "\\",
            files: [
              {
                path: "guide/intro.md",
                name: "intro.md",
                title: "Intro",
              },
            ],
          },
        ],
        defaultDocument: { root: "r0", path: "README.md" },
      }),
      getDocument: vi.fn().mockImplementation(async (path: string) => ({
        path,
        title: path.startsWith("r1/") ? "Intro" : "Alpha Readme",
        html: `<h1>${path.startsWith("r1/") ? "Intro" : "Alpha"}</h1>`,
      })),
    });
    const view = render(<App api={workspaceAPI} />);
    await screen.findByRole("heading", { level: 1, name: "Alpha" });

    await user.hover(screen.getByRole("button", { name: "beta" }));
    expect(
      await screen.findByText("D:\\work\\beta", {
        selector: ".tree-tooltip-path",
      }),
    ).toBeTruthy();
    await user.unhover(screen.getByRole("button", { name: "beta" }));

    await user.hover(screen.getByRole("button", { name: "r1/guide" }));
    expect(
      await screen.findByText("D:\\work\\beta\\guide", {
        selector: ".tree-tooltip-path",
      }),
    ).toBeTruthy();
    view.unmount();
  });

  it("restores sidebar layout but removes the legacy stored document width", async () => {
    const user = userEvent.setup();
    window.localStorage.setItem(
      "m2h.preview.layout",
      JSON.stringify({
        sidebarOpen: false,
        sidebarWidth: 420,
        documentWidth: "wide",
      }),
    );
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    expect(document.querySelector(".reader-canvas-standard")).toBeTruthy();
    expect(
      JSON.parse(window.localStorage.getItem("m2h.preview.layout") ?? "{}"),
    ).not.toHaveProperty("documentWidth");
    expect(
      document
        .querySelector('[data-slot="sidebar-wrapper"]')
        ?.getAttribute("style"),
    ).toContain("420px");
    expect(
      document
        .querySelector('[data-slot="sidebar"]')
        ?.getAttribute("data-state"),
    ).toBe("collapsed");
    await user.hover(screen.getByRole("button", { name: "切换文件导航" }));
    expect(await screen.findByText("展开文件导航")).toBeTruthy();
  });

  it("shows empty, API, deleted-document, and attachment errors", async () => {
    let view = render(
      <App
        api={createAPI({
          listFiles: vi.fn().mockResolvedValue({
            kind: "directory",
            version: "0.9.1",
            roots: [
              {
                id: "r0",
                name: "docs",
                kind: "directory",
                absolutePath: "/tmp/docs",
                pathSeparator: "/",
                files: [],
              },
            ],
            defaultDocument: null,
          }),
        })}
      />,
    );
    expect(await screen.findAllByText("目录中没有 Markdown 文件")).toHaveLength(
      2,
    );
    expect(window.location.pathname + window.location.search).toBe("/");
    view.unmount();

    view = render(
      <App
        api={createAPI({
          listFiles: vi.fn().mockRejectedValue(new Error("offline")),
        })}
      />,
    );
    const listAlert = await screen.findByRole("alert");
    expect(listAlert.textContent).toContain("无法读取 Markdown 文件列表");
    view.unmount();

    view = render(
      <App
        api={createAPI({
          getDocument: vi
            .fn()
            .mockRejectedValue(new APIError(404, "not found")),
        })}
      />,
    );
    const deletionAlert = await screen.findByRole("alert");
    expect(deletionAlert.textContent).toContain("文档已被删除");
    view.unmount();

    render(
      <App
        api={createAPI({
          getDocument: vi.fn().mockResolvedValue({
            path: "README.md",
            title: "Readme API Title",
            html: '<img src="/assets/missing.png" alt="missing diagram">',
          }),
        })}
      />,
    );
    const image = await screen.findByRole("img", { name: "missing diagram" });
    fireEvent.error(image);
    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain(
        "/assets/missing.png",
      ),
    );
  });

  it("shows the empty-document state for blank bodies while keeping frontmatter", async () => {
    // Case A: a zero-byte Markdown file renders an empty body.
    let view = render(
      <App
        api={createAPI({
          getDocument: vi.fn().mockResolvedValue({
            path: "README.md",
            title: "Readme API Title",
            html: "",
            frontmatter: null,
            toc: [],
          }),
        })}
      />,
    );
    expect(await screen.findByText("当前文档无内容")).toBeTruthy();
    expect(document.querySelector(".reader-document")).toBeNull();
    view.unmount();

    // Case B: whitespace-only Markdown is just as empty.
    view = render(
      <App
        api={createAPI({
          getDocument: vi.fn().mockResolvedValue({
            path: "README.md",
            title: "Readme API Title",
            html: " \n ",
            frontmatter: null,
            toc: [],
          }),
        })}
      />,
    );
    expect(await screen.findByText("当前文档无内容")).toBeTruthy();
    view.unmount();

    // Case C: frontmatter over an empty body keeps the metadata panel above
    // the empty state instead of hiding it.
    view = render(
      <App
        api={createAPI({
          getDocument: vi.fn().mockResolvedValue({
            path: "README.md",
            title: "Readme API Title",
            html: "",
            frontmatter: {
              entries: [{ key: "author", value: "lzwang" }],
            },
            toc: [],
          }),
        })}
      />,
    );
    expect(await screen.findByText("当前文档无内容")).toBeTruthy();
    expect(screen.getByText("Frontmatter")).toBeTruthy();
    view.unmount();

    // Case D: a document with a real body never shows the empty state.
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");
    expect(screen.queryByText("当前文档无内容")).toBeNull();
  });

  it("renders frontmatter summary and a collapsed panel", async () => {
    const user = userEvent.setup();
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h1 id="top">Readme</h1>',
        frontmatter: {
          entries: [
            { key: "date", value: "2026-07-11" },
            { key: "tags", value: "- Go\n- Markdown" },
            { key: "author", value: "lzwang" },
          ],
          date: "2026-07-11",
          tags: ["Go", "Markdown"],
        },
      }),
    });
    render(<App api={api} />);

    await screen.findByRole("heading", { name: "Readme", level: 1 });

    // Toolbar summary surfaces the normalized date and tags.
    const titleRegion = screen.getByRole("region", { name: "当前文档标题" });
    expect(within(titleRegion).getByText("2026-07-11")).toBeTruthy();
    expect(within(titleRegion).getByText("Go · Markdown")).toBeTruthy();

    // The frontmatter panel is present but collapsed by default.
    const panel = screen.getByText("Frontmatter").closest("details");
    if (panel === null) {
      throw new Error("frontmatter panel was not rendered");
    }
    expect(panel.hasAttribute("open")).toBe(false);
    expect(within(panel).getByText("3")).toBeTruthy();

    // Expanding opens the panel.
    await user.click(screen.getByText("Frontmatter"));
    expect(panel.hasAttribute("open")).toBe(true);
  });

  it("keeps the panel but hides the summary without date or tags", async () => {
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h1 id="top">Readme</h1>',
        frontmatter: {
          entries: [{ key: "author", value: "lzwang" }],
        },
      }),
    });
    render(<App api={api} />);

    await screen.findByRole("heading", { name: "Readme", level: 1 });

    expect(screen.queryByText("2026-07-11")).toBeNull();
    expect(screen.queryByText("Frontmatter")).toBeTruthy();
  });

  it("renders no frontmatter UI when metadata is absent", async () => {
    render(<App api={createAPI()} />);

    await screen.findByText("Body for README.md");
    expect(screen.queryByText("Frontmatter")).toBeNull();
  });

  it("preserves the Markdown DOM across theme changes", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const article = document.querySelector<HTMLElement>(".reader-document");
    if (article === null) {
      throw new Error("reader article was not rendered");
    }
    const paragraph = article.querySelector("p");
    if (paragraph === null) {
      throw new Error("document paragraph was not rendered");
    }

    await user.click(
      screen.getByRole("button", { name: "显示主题：跟随系统" }),
    );
    await user.click(
      await screen.findByRole("menuitemradio", { name: "深色" }),
    );

    // The article body is not rebuilt on a theme switch: the same paragraph
    // node survives. Only Mermaid SVGs regenerate (their colors are baked in),
    // which is covered at the renderer level — DOM identity, not innerHTML
    // equality, is the regression signal.
    expect(article.querySelector("p")).toBe(paragraph);
    expect(article.textContent).toContain("Body for README.md");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    // The stylesheet URL is stable across a theme switch: no ?mode= change, so
    // there is no new Markdown CSS request and no full-body style recalculation.
    expect(
      document.getElementById("m2h-markdown-styles")?.getAttribute("href"),
    ).toBe("/ui/markdown.css");
  });

  it("keeps focus on an in-body link when the resolved theme changes", async () => {
    const themeMedia = installThemeMedia(false);
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<p>read <a href="https://example.com">external link</a> here</p>',
        frontmatter: null,
        toc: [],
      }),
    });
    render(<App api={api} />);

    const link = await screen.findByRole("link", { name: "external link" });
    const article = document.querySelector<HTMLElement>(".reader-document");
    if (article === null) {
      throw new Error("reader article was not rendered");
    }
    const paragraph = article.querySelector("p");
    if (paragraph === null) {
      throw new Error("document paragraph was not rendered");
    }

    link.focus();
    expect(document.activeElement).toBe(link);

    // Flip the system preference while in "auto" mode: resolvedMode changes
    // but the article is not rebuilt, so the focused link and the paragraph
    // node keep their identity and focus.
    await act(async () => {
      themeMedia.setMatches(true);
    });

    expect(document.activeElement).toBe(link);
    expect(article.querySelector("p")).toBe(paragraph);
  });

  it("preserves enhanced Markdown DOM across width changes", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const article = document.querySelector<HTMLElement>(".reader-document");
    if (article === null) {
      throw new Error("reader article was not rendered");
    }

    article.innerHTML = '<div data-rich-enhanced="true">enhanced content</div>';

    await user.click(screen.getByRole("button", { name: "文档宽度：标准" }));
    await user.click(await screen.findByRole("menuitemradio", { name: "宽" }));

    expect(article.querySelector('[data-rich-enhanced="true"]')).not.toBeNull();
    expect(article.textContent).toBe("enhanced content");
  });

  it("renders the table of contents with H2-H4 entries and highlights the active heading", async () => {
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h1 id="top">Readme</h1><h2 id="install">Install</h2><h3 id="homebrew">Homebrew</h3><p>body</p>',
        frontmatter: null,
        toc: [
          { level: 1, id: "top", text: "Readme" },
          { level: 2, id: "install", text: "Install" },
          { level: 3, id: "homebrew", text: "Homebrew" },
          { level: 5, id: "deep", text: "Deep" },
        ],
      }),
    });
    render(<App api={api} />);

    const nav = await screen.findByRole("navigation", { name: "文档目录" });
    // H1 (already the document title) and H5 (too deep) are excluded.
    expect(within(nav).getByRole("link", { name: "Install" })).toBeTruthy();
    expect(within(nav).getByRole("link", { name: "Homebrew" })).toBeTruthy();
    expect(within(nav).queryByRole("link", { name: "Readme" })).toBeNull();
    expect(within(nav).queryByRole("link", { name: "Deep" })).toBeNull();

    expect(screen.getByRole("button", { name: "隐藏文档目录" })).toBeTruthy();

    // Pinned to the top of the document, no section is active yet so the URL
    // stays clean and no TOC link reports the current location.
    expect(within(nav).queryByRole("link", { current: "location" })).toBeNull();

    // Scrolling partway down: every heading sits at top:0 in jsdom, so the last
    // heading (the H5) becomes the active position and the TOC highlights its
    // nearest H2–H4 ancestor.
    await driveWindowScroll(100);
    expect(
      await screen.findByRole("link", {
        name: "Homebrew",
        current: "location",
      }),
    ).toBeTruthy();
  });

  it("toggles the table of contents off and on through the toolbar", async () => {
    const user = userEvent.setup();
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="install">Install</h2>',
        frontmatter: null,
        toc: [{ level: 2, id: "install", text: "Install" }],
      }),
    });
    render(<App api={api} />);
    await screen.findByRole("navigation", { name: "文档目录" });
    expect(window.location.search).toBe("");

    await user.click(screen.getByRole("button", { name: "隐藏文档目录" }));
    expect(screen.queryByRole("navigation", { name: "文档目录" })).toBeNull();
    expect(window.location.search).toBe("?toc=false");

    await user.click(screen.getByRole("button", { name: "显示文档目录" }));
    expect(
      await screen.findByRole("navigation", { name: "文档目录" }),
    ).toBeTruthy();
    expect(window.location.search).toBe("");
  });

  it("disables the TOC toggle and omits the panel without H2-H4 headings", async () => {
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const toggle = screen.getByRole("button", { name: "当前文档没有目录" });
    expect((toggle as HTMLButtonElement).disabled).toBe(true);
    expect(screen.queryByRole("navigation", { name: "文档目录" })).toBeNull();
    // The narrow-screen sheet trigger follows the same availability rule.
    expect(screen.queryByRole("button", { name: "打开文档目录" })).toBeNull();
  });

  it("opens the narrow-screen TOC sheet, navigates, and leaves the rail state alone", async () => {
    const user = userEvent.setup();
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="install">Install</h2>',
        frontmatter: null,
        toc: [{ level: 2, id: "install", text: "Install" }],
      }),
    });
    render(<App api={api} />);

    await user.click(
      await screen.findByRole("button", { name: "打开文档目录" }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("link", { name: "Install" }));

    // The sheet hands the scroll to the next frame after closing; the shared
    // heading navigator then records the fragment through the URL funnel.
    await expect.poll(() => window.location.hash).toBe("#install");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    // The sheet is a transient UI: the persistent rail state stays untouched.
    expect(window.location.search).toBe("");
    expect(screen.getByRole("button", { name: "隐藏文档目录" })).toBeTruthy();
  });

  it("scrolls to a Unicode heading and records the hash when a TOC entry is clicked", async () => {
    const user = userEvent.setup();
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="安装">安装</h2>',
        frontmatter: null,
        toc: [{ level: 2, id: "安装", text: "安装" }],
      }),
    });
    render(<App api={api} />);
    const link = await screen.findByRole("link", { name: "安装" });
    await user.click(link);
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith(
      expect.objectContaining({ block: "start" }),
    );
    expect(window.location.hash).toBe(`#${encodeURIComponent("安装")}`);
  });

  it("does not reload the document for a same-document fragment link", async () => {
    const user = userEvent.setup();
    const getDocument = vi.fn().mockResolvedValue({
      path: "README.md",
      title: "Readme API Title",
      html: '<h2 id="section">Section</h2><p><a href="#section">jump</a></p>',
      frontmatter: null,
      toc: [{ level: 2, id: "section", text: "Section" }],
    });
    const api = createAPI({ getDocument });
    render(<App api={api} />);
    await screen.findByRole("heading", { level: 2, name: "Section" });
    expect(getDocument).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("link", { name: "jump" }));
    // The fragment resolves inside the open document, so it scrolls in place and
    // updates the hash without fetching the document again.
    expect(getDocument).toHaveBeenCalledTimes(1);
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith(
      expect.objectContaining({ block: "start" }),
    );
    expect(window.location.pathname + window.location.hash).toBe(
      "/doc/README.md#section",
    );
  });

  it("syncs the URL hash to the active heading while scrolling, even with the TOC hidden", async () => {
    const user = userEvent.setup();
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="alpha">Alpha</h2><h2 id="beta">Beta</h2>',
        frontmatter: null,
        toc: [
          { level: 2, id: "alpha", text: "Alpha" },
          { level: 2, id: "beta", text: "Beta" },
        ],
      }),
    });
    render(<App api={api} />);
    await screen.findByRole("heading", { level: 2, name: "Alpha" });
    // Pinned at the top, the URL stays clean.
    expect(window.location.hash).toBe("");

    // With the TOC panel hidden the URL must still follow the reading position.
    await user.click(screen.getByRole("button", { name: "隐藏文档目录" }));

    // Scrolled partway down: every heading sits at top:0 in jsdom, so the spy
    // reports the last heading "beta" and the URL follows via replaceState.
    await settleScrollPosition();
    expect(window.location.hash).toBe("#beta");
    expect(window.location.search).toBe("?toc=false");
  });

  it("does not reload when a heading permalink is clicked", async () => {
    const getDocument = vi.fn().mockResolvedValue({
      path: "README.md",
      title: "Readme API Title",
      html: '<h2 id="section">Section</h2>',
      frontmatter: null,
      toc: [{ level: 2, id: "section", text: "Section" }],
    });
    const api = createAPI({ getDocument });
    render(<App api={api} />);
    await screen.findByRole("heading", { level: 2, name: "Section" });
    const anchor = document.querySelector(".markdown-body .m2h-heading-anchor");
    if (!(anchor instanceof HTMLAnchorElement)) {
      throw new Error("heading permalink was not rendered");
    }
    expect(anchor.getAttribute("href")).toBe("#section");

    vi.mocked(Element.prototype.scrollIntoView).mockClear();
    await act(async () => {
      fireEvent.click(anchor);
    });
    // A permalink is a same-document link: it scrolls and updates the hash
    // without fetching the document again.
    expect(getDocument).toHaveBeenCalledTimes(1);
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith(
      expect.objectContaining({ block: "start" }),
    );
    expect(window.location.hash).toBe("#section");
  });

  it("locates a Unicode heading from a deep link", async () => {
    window.history.replaceState(
      null,
      "",
      `/doc/README.md#${encodeURIComponent("安装")}`,
    );
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="安装">安装</h2>',
        frontmatter: null,
        toc: [{ level: 2, id: "安装", text: "安装" }],
      }),
    });
    render(<App api={api} />);
    await screen.findByRole("heading", { level: 2, name: "安装" });
    await waitFor(() =>
      expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith(
        expect.objectContaining({ block: "start" }),
      ),
    );
    await settleScrollPosition();
    expect(window.location.hash).toBe(`#${encodeURIComponent("安装")}`);
  });

  it("locates a duplicate heading by its suffixed id", async () => {
    window.history.replaceState(null, "", "/doc/README.md#foo-1");
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="foo">First</h2><h2 id="foo-1">Second</h2>',
        frontmatter: null,
        toc: [
          { level: 2, id: "foo", text: "First" },
          { level: 2, id: "foo-1", text: "Second" },
        ],
      }),
    });
    render(<App api={api} />);
    await screen.findByRole("heading", { level: 2, name: "Second" });
    await waitFor(() =>
      expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith(
        expect.objectContaining({ block: "start" }),
      ),
    );
    await settleScrollPosition();
    expect(window.location.hash).toBe("#foo-1");
  });

  it("restores the saved pixel offset after a reload, ignoring the fragment", async () => {
    // A reload must return to the exact pixel the tab saved for the document —
    // the URL fragment only names the enclosing section. Model the
    // NavigationTiming entry a reload produces, plus the saved offset.
    vi.spyOn(performance, "getEntriesByType").mockReturnValue([
      { type: "reload" } as unknown as PerformanceEntry,
    ]);
    saveScrollPosition("guides/setup.md", 4287);
    window.history.replaceState(null, "", "/doc/guides/setup.md#install");
    // scrollIntoView is a shared mock in the test setup; reset it so the count
    // reflects only this test's landing decision.
    vi.mocked(Element.prototype.scrollIntoView).mockClear();
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "guides/setup.md",
        title: "Setup API Title",
        html: '<h2 id="install">Install</h2>',
        frontmatter: null,
        toc: [{ level: 2, id: "install", text: "Install" }],
      }),
    });
    const scrollTo = vi.fn();
    Object.defineProperty(window, "scrollTo", {
      configurable: true,
      writable: true,
      value: scrollTo,
    });
    render(<App api={api} />);
    await screen.findByRole("heading", { level: 2, name: "Install" });
    expect(scrollTo).toHaveBeenCalledWith(0, 4287);
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
    // The URL keeps its fragment: the scroll spy resyncs it only from real
    // scroll events once the restore lands.
    expect(window.location.hash).toBe("#install");
  });

  it("falls back to the fragment after a reload with nothing saved", async () => {
    vi.spyOn(performance, "getEntriesByType").mockReturnValue([
      { type: "reload" } as unknown as PerformanceEntry,
    ]);
    window.history.replaceState(null, "", "/doc/guides/setup.md#install");
    vi.mocked(Element.prototype.scrollIntoView).mockClear();
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "guides/setup.md",
        title: "Setup API Title",
        html: '<h2 id="install">Install</h2>',
        frontmatter: null,
        toc: [{ level: 2, id: "install", text: "Install" }],
      }),
    });
    render(<App api={api} />);
    await screen.findByRole("heading", { level: 2, name: "Install" });
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith(
      expect.objectContaining({ block: "start" }),
    );
  });

  it("persists the window scroll offset per document and flushes it on pagehide", async () => {
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="install">Install</h2>',
        frontmatter: null,
        toc: [{ level: 2, id: "install", text: "Install" }],
      }),
    });
    render(<App api={api} />);
    await screen.findByRole("heading", { level: 2, name: "Install" });
    // A scroll drives the rAF-throttled saver (rAF is setTimeout(0) in the test
    // setup, so one macrotask turn is enough).
    await driveWindowScroll(250);
    expect(readScrollPosition("README.md")).toBe(250);
    // A scroll followed immediately by pagehide never runs its rAF: the flush
    // must persist the offset synchronously.
    await act(async () => {
      Object.defineProperty(window, "scrollY", {
        configurable: true,
        value: 310,
      });
      fireEvent.scroll(window);
      window.dispatchEvent(new Event("pagehide"));
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    });
    expect(readScrollPosition("README.md")).toBe(310);
  });

  it("does not re-jump to the fragment when the same document is hot-swapped", async () => {
    // After the initial fragment landing, the URL hash follows the reading
    // position. A server-sent body hot-swap of the same document must not
    // treat that hash as a fresh deep link: the never-unmounted ScrollArea
    // keeps the offset and scroll anchoring absorbs the reflow.
    const getDocument = vi
      .fn<PreviewAPI["getDocument"]>()
      .mockResolvedValueOnce({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="install">Install</h2><p>Original body</p>',
        frontmatter: null,
        toc: [{ level: 2, id: "install", text: "Install" }],
      })
      .mockResolvedValueOnce({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="install">Install</h2><p>Updated body</p>',
        frontmatter: null,
        toc: [{ level: 2, id: "install", text: "Install" }],
      });
    const api = createAPI({ getDocument });
    const events = stubEventSource();
    render(<App api={api} />);

    await screen.findByText("Original body");
    // The reader scrolled: the spy wrote the section into the URL.
    window.history.replaceState(null, "", "/doc/README.md#install");
    vi.mocked(Element.prototype.scrollIntoView).mockClear();

    await act(async () => {
      events.dispatch("document-changed");
    });
    await screen.findByText("Updated body");
    expect(getDocument).toHaveBeenCalledTimes(2);
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
  });

  it("shows floating jump buttons that reflect the scroll boundaries", async () => {
    // Model a tall document: jsdom reports no layout, so the scrollable height
    // is stubbed directly on the scrolling element. Earlier tests leave the
    // window mid-scroll, so pin the viewport to the top before mounting.
    Object.defineProperty(document.documentElement, "scrollHeight", {
      configurable: true,
      value: 4000,
    });
    await driveWindowScroll(0);
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const top = screen.getByRole("button", { name: "回到顶部" });
    const bottom = screen.getByRole("button", { name: "前往底部" });
    // Pinned at the very top, jumping up is a no-op.
    expect((top as HTMLButtonElement).disabled).toBe(true);
    expect((bottom as HTMLButtonElement).disabled).toBe(false);

    // Mid-document: both edges are reachable.
    await driveWindowScroll(2000);
    expect((top as HTMLButtonElement).disabled).toBe(false);
    expect((bottom as HTMLButtonElement).disabled).toBe(false);

    // At the bottom edge the downward jump disables instead.
    await driveWindowScroll(4000);
    expect((top as HTMLButtonElement).disabled).toBe(false);
    expect((bottom as HTMLButtonElement).disabled).toBe(true);
  });

  it("jumps to the top and bottom with smooth window scrolling", async () => {
    const user = userEvent.setup();
    Object.defineProperty(document.documentElement, "scrollHeight", {
      configurable: true,
      value: 4000,
    });
    const scrollTo = vi.fn();
    Object.defineProperty(window, "scrollTo", {
      configurable: true,
      writable: true,
      value: scrollTo,
    });
    await driveWindowScroll(0);
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");
    // Start mid-document so both jumps are enabled.
    await driveWindowScroll(2000);

    await user.click(screen.getByRole("button", { name: "前往底部" }));
    expect(scrollTo).toHaveBeenCalledWith({ top: 4000, behavior: "smooth" });

    await user.click(screen.getByRole("button", { name: "回到顶部" }));
    expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: "smooth" });
  });

  it("disables both jump buttons when the document fits the viewport", async () => {
    // No scrollable overflow (jsdom's own geometry) and a fresh viewport:
    // both edges are already reached.
    Object.defineProperty(document.documentElement, "scrollHeight", {
      configurable: true,
      value: 0,
    });
    await driveWindowScroll(0);
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    // jsdom's document has no scrollable overflow: both edges are already
    // reached, so neither jump is offered.
    expect(
      (screen.getByRole("button", { name: "回到顶部" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
    expect(
      (screen.getByRole("button", { name: "前往底部" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
  });

  it("refreshes the jump states when late content grows the document", async () => {
    // Capture ResizeObserver callbacks so the test can deliver the resize a
    // client-rendered body growth fires — no scroll or resize event at all.
    const callbacks: ResizeObserverCallback[] = [];
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(callback: ResizeObserverCallback) {
          callbacks.push(callback);
        }
        observe(): void {}
        unobserve(): void {}
        disconnect(): void {}
      },
    );
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const bottom = screen.getByRole("button", {
      name: "前往底部",
    }) as HTMLButtonElement;
    // Mount-time geometry: the (empty) document fits the viewport.
    expect(bottom.disabled).toBe(true);

    // The body renders after mount and grows far past the viewport; the
    // document resize must flip the bottom jump back on.
    Object.defineProperty(document.documentElement, "scrollHeight", {
      configurable: true,
      value: 4000,
    });
    await act(async () => {
      for (const callback of callbacks) {
        callback([], {} as ResizeObserver);
      }
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    });
    expect(bottom.disabled).toBe(false);
  });

  it("lands a deep link into the second root and switches roots in place", async () => {
    window.history.replaceState(null, "", "/doc/r1/README.md#section");
    const getDocument = vi.fn().mockImplementation(async (path: string) => ({
      path,
      title: path.startsWith("r1/") ? "Beta Readme" : "Alpha Readme",
      html: path.startsWith("r1/")
        ? '<h2 id="section">Section</h2>'
        : "<p>Alpha body</p>",
      frontmatter: null,
      toc: path.startsWith("r1/")
        ? [{ level: 2, id: "section", text: "Section" }]
        : [],
    }));
    const api = createAPI({
      listFiles: vi.fn().mockResolvedValue({
        kind: "workspace",
        version: "0.9.1",
        roots: [
          {
            id: "r0",
            name: "alpha",
            kind: "directory",
            absolutePath: "/tmp/alpha",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Alpha Readme" },
            ],
          },
          {
            id: "r1",
            name: "beta",
            kind: "directory",
            absolutePath: "/tmp/beta",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Beta Readme" },
            ],
          },
        ],
        defaultDocument: { root: "r0", path: "README.md" },
      }),
      getDocument,
    });
    render(<App api={api} />);

    // A fresh #hash link into the second root opens that root's same-named
    // document (not the primary's) and scrolls to the section.
    await screen.findByRole("heading", { level: 2, name: "Section" });
    expect(getDocument).toHaveBeenCalledWith(
      "r1/README.md",
      expect.any(AbortSignal),
    );
    await waitFor(() =>
      expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith(
        expect.objectContaining({ block: "start" }),
      ),
    );
    expect(window.location.pathname + window.location.hash).toBe(
      "/doc/r1/README.md#section",
    );

    // Switching to the first root's copy pushes its own virtual path.
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Alpha Readme，r0/README.md" }),
    );
    await screen.findByText("Alpha body");
    expect(window.location.pathname).toBe("/doc/r0/README.md");
  });

  it("restores per-root scroll positions independently after a reload", async () => {
    // The two roots hold same-named documents; their saved offsets must never
    // leak across the virtual key boundary.
    vi.spyOn(performance, "getEntriesByType").mockReturnValue([
      { type: "reload" } as unknown as PerformanceEntry,
    ]);
    saveScrollPosition("r0/README.md", 111);
    saveScrollPosition("r1/README.md", 222);
    window.history.replaceState(null, "", "/doc/r1/README.md");
    const getDocument = vi.fn().mockImplementation(async (path: string) => ({
      path,
      title: path.startsWith("r1/") ? "Beta Readme" : "Alpha Readme",
      html: `<p>Body for ${path}</p>`,
      frontmatter: null,
      toc: [],
    }));
    const api = createAPI({
      listFiles: vi.fn().mockResolvedValue({
        kind: "workspace",
        version: "0.9.1",
        roots: [
          {
            id: "r0",
            name: "alpha",
            kind: "directory",
            absolutePath: "/tmp/alpha",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Alpha Readme" },
            ],
          },
          {
            id: "r1",
            name: "beta",
            kind: "directory",
            absolutePath: "/tmp/beta",
            pathSeparator: "/",
            files: [
              { path: "README.md", name: "README.md", title: "Beta Readme" },
            ],
          },
        ],
        defaultDocument: { root: "r0", path: "README.md" },
      }),
      getDocument,
    });
    const scrollTo = vi.fn();
    Object.defineProperty(window, "scrollTo", {
      configurable: true,
      writable: true,
      value: scrollTo,
    });
    render(<App api={api} />);

    // The reloaded tab returns to the second root's own pixel offset.
    await screen.findByText("Body for r1/README.md");
    expect(scrollTo).toHaveBeenCalledWith(0, 222);

    // Switching documents restores the first root's own offset.
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("button", { name: "Alpha Readme，r0/README.md" }),
    );
    await screen.findByText("Body for r0/README.md");
    expect(scrollTo).toHaveBeenCalledWith(0, 111);
    expect(readScrollPosition("r1/README.md")).toBe(222);
  });

  it("keeps toc and width query parameters independent", async () => {
    const user = userEvent.setup();
    const api = createAPI({
      getDocument: vi.fn().mockResolvedValue({
        path: "README.md",
        title: "Readme API Title",
        html: '<h2 id="install">Install</h2>',
        frontmatter: null,
        toc: [{ level: 2, id: "install", text: "Install" }],
      }),
    });
    render(<App api={api} />);
    await screen.findByRole("navigation", { name: "文档目录" });

    await user.click(screen.getByRole("button", { name: "隐藏文档目录" }));
    await user.click(screen.getByRole("button", { name: "文档宽度：标准" }));
    await user.click(await screen.findByRole("menuitemradio", { name: "宽" }));
    expect(window.location.search).toBe("?width=wide&toc=false");
  });
});

function createAPI(overrides: Partial<PreviewAPI> = {}): PreviewAPI {
  return {
    listFiles: vi.fn().mockResolvedValue(initialFiles),
    getDocument: vi.fn().mockImplementation(async (path: string) => {
      const file = initialFiles.roots[0]?.files.find(
        (candidate) => candidate.path === path,
      );
      if (file === undefined) {
        throw new APIError(404, "not found");
      }
      return {
        path,
        title: file.title,
        html: `<p>Body for ${path}</p>`,
        frontmatter: null,
        toc: [],
      };
    }),
    getMarkdown: vi.fn().mockImplementation(async (path: string) => {
      const file = initialFiles.roots[0]?.files.find(
        (candidate) => candidate.path === path,
      );
      if (file === undefined) {
        throw new APIError(404, "not found");
      }
      return `# Raw source of ${path}\n`;
    }),
    ...overrides,
  };
}

// jsdom does not reflect layout or scroll offsets, so a real "scrolled to a
// heading" state never reaches the heading spy. Model it explicitly (a
// mid-document offset plus a scroll event on the window, which is the scroller)
// so the spy reports the active section and the URL-sync logic settles before
// assertions read the hash.
async function driveWindowScroll(offset: number): Promise<void> {
  Object.defineProperty(window, "scrollY", {
    configurable: true,
    value: offset,
  });
  await act(async () => {
    fireEvent.scroll(window);
    await new Promise((resolve) => {
      setTimeout(resolve, 0);
    });
  });
}

async function settleScrollPosition(): Promise<void> {
  await driveWindowScroll(100);
}

// installThemeMedia swaps window.matchMedia for a controllable prefers-color-
// scheme query so a test can flip the system theme (and thus resolvedMode in
// "auto" mode) without going through the theme menu — exercising the same
// resolved-mode change path while leaving document focus untouched.
function installThemeMedia(initialMatches: boolean): {
  setMatches(matches: boolean): void;
} {
  const listeners = new Set<(event: { matches: boolean }) => void>();
  const media = {
    matches: initialMatches,
    media: "(prefers-color-scheme: dark)",
    onchange: null as ((event: { matches: boolean }) => void) | null,
    addEventListener(
      _type: string,
      listener: (event: { matches: boolean }) => void,
    ): void {
      listeners.add(listener);
    },
    removeEventListener(
      _type: string,
      listener: (event: { matches: boolean }) => void,
    ): void {
      listeners.delete(listener);
    },
    addListener(listener: (event: { matches: boolean }) => void): void {
      listeners.add(listener);
    },
    removeListener(listener: (event: { matches: boolean }) => void): void {
      listeners.delete(listener);
    },
    dispatchEvent(): boolean {
      return true;
    },
  };
  vi.spyOn(window, "matchMedia").mockImplementation((query: string) => {
    if (query === "(prefers-color-scheme: dark)") {
      return media as unknown as MediaQueryList;
    }
    return { ...media, media: query } as unknown as MediaQueryList;
  });
  return {
    setMatches(matches: boolean) {
      media.matches = matches;
      for (const listener of listeners) {
        listener({ matches });
      }
    },
  };
}

// stubEventSource replaces window.EventSource with a mock the test can dispatch
// onto. It mirrors the real EventSource surface used by usePreviewEvents.
function stubEventSource(): { dispatch(type: string): void } {
  const sources: Array<{
    dispatch(type: string): void;
  }> = [];

  class DispatchableEventSource {
    readonly url: string;
    private readonly listeners = new Map<string, Set<(event: Event) => void>>();

    constructor(url: string) {
      this.url = url;
      sources.push(this);
    }

    addEventListener(type: string, listener: (event: Event) => void): void {
      let listeners = this.listeners.get(type);
      if (listeners === undefined) {
        listeners = new Set();
        this.listeners.set(type, listeners);
      }
      listeners.add(listener);
    }

    removeEventListener(type: string, listener: (event: Event) => void): void {
      this.listeners.get(type)?.delete(listener);
    }

    close(): void {
      // No-op: real EventSource teardown is irrelevant to these tests.
    }

    dispatch(type: string): void {
      const event = new Event(type);
      this.listeners.get(type)?.forEach((listener) => {
        listener(event);
      });
    }
  }

  vi.stubGlobal("EventSource", DispatchableEventSource);
  return {
    dispatch(type: string) {
      sources.forEach((source) => {
        source.dispatch(type);
      });
    },
  };
}
