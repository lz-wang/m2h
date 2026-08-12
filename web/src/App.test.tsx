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

const initialFiles: FileListResponse = {
  kind: "directory",
  version: "0.9.1",
  files: [
    { path: "README.md", name: "README.md", title: "Readme API Title" },
    { path: "guides/setup.md", name: "setup.md", title: "Setup API Title" },
  ],
  defaultPath: "README.md",
};

beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
  document.getElementById("m2h-markdown-styles")?.remove();
  document.documentElement.className = "";
  delete document.documentElement.dataset.mode;
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
    ).toBe("/ui/markdown.css?mode=auto");
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
        files: [
          { path: "README.md", name: "README.md", title: "Readme API Title" },
        ],
        defaultPath: "README.md",
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
    ).toBe("/ui/markdown.css?mode=dark");
    expect(window.location.hash).toBe("#install");
    await waitFor(() =>
      expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({
        block: "start",
      }),
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
        files: [file],
        defaultPath: path,
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
            files: [],
            defaultPath: "",
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

  it("preserves enhanced Markdown DOM across theme changes", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const article = document.querySelector<HTMLElement>(".reader-document");
    if (article === null) {
      throw new Error("reader article was not rendered");
    }

    // Simulate KaTeX / Mermaid replacing the original Markdown DOM. Any UI
    // state change must not reset this imperative enhancement.
    article.innerHTML = '<div data-rich-enhanced="true">enhanced content</div>';

    await user.click(
      screen.getByRole("button", { name: "显示主题：跟随系统" }),
    );
    await user.click(
      await screen.findByRole("menuitemradio", { name: "深色" }),
    );

    expect(article.querySelector('[data-rich-enhanced="true"]')).not.toBeNull();
    expect(article.textContent).toBe("enhanced content");
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

    // In jsdom every heading sits at top:0, so the scroll spy activates the
    // last entry that has scrolled past the viewport top.
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
      const file = initialFiles.files.find(
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
    ...overrides,
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
