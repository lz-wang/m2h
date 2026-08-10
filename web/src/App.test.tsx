import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { APIError, type FileListResponse, type PreviewAPI } from "./api";

const initialFiles: FileListResponse = {
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
      "/doc/README.md?mode=auto",
    );
    expect(document.title).toBe("Readme API Title");
    expect(document.documentElement.dataset.mode).toBe("auto");
    expect(
      document.getElementById("m2h-markdown-styles")?.getAttribute("href"),
    ).toBe("/ui/markdown.css?mode=auto");
    expect(screen.getByText("2 个 Markdown 文件")).toBeTruthy();
    const title = screen.getByRole("region", { name: "当前文档标题" });
    expect(title.textContent).toBe("Readme API Title");
    expect(title.getAttribute("title")).toBe("README.md");
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
    ).toBe("/doc/guides/setup.md?mode=auto#install");

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
      "/doc/guides/setup.md?mode=auto",
    );

    window.history.replaceState(null, "", "/doc/README.md?mode=light");
    window.dispatchEvent(new PopStateEvent("popstate"));
    await screen.findByText("Body for README.md");
    expect(screen.getByRole("button", { name: "显示主题：浅色" })).toBeTruthy();
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("keeps the current document on refresh and falls back after deletion", async () => {
    const user = userEvent.setup();
    const listFiles = vi
      .fn<PreviewAPI["listFiles"]>()
      .mockResolvedValueOnce(initialFiles)
      .mockResolvedValueOnce(initialFiles)
      .mockResolvedValueOnce({
        files: initialFiles.files.filter((file) => file.path === "README.md"),
        defaultPath: "README.md",
      });
    const api = createAPI({ listFiles });
    render(<App api={api} />);
    await screen.findByText("Body for README.md");

    await user.click(screen.getByRole("button", { name: "guides" }));
    await user.click(
      screen.getByRole("button", { name: "Setup API Title，guides/setup.md" }),
    );
    await screen.findByText("Body for guides/setup.md");

    await user.click(screen.getByRole("button", { name: "刷新文件列表" }));
    await screen.findByText("Body for guides/setup.md");
    expect(window.location.pathname).toBe("/doc/guides/setup.md");

    await user.click(screen.getByRole("button", { name: "刷新文件列表" }));
    await screen.findByText("Body for README.md");
    expect(window.location.pathname + window.location.search).toBe(
      "/doc/README.md?mode=auto",
    );
    expect(listFiles).toHaveBeenCalledTimes(3);
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

    expect(
      screen
        .getByRole("button", { name: "显示主题：跟随系统" })
        .querySelector(".lucide-monitor-cog"),
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

  it("exposes full file names and resizes the desktop sidebar by dragging", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");

    const file = screen.getByRole("button", {
      name: "Readme API Title，README.md",
    });
    await user.hover(file);
    const fileTooltip = await screen.findByRole("tooltip");
    expect(fileTooltip.textContent).toContain("README.md");
    expect(fileTooltip.textContent).toContain("Readme API Title");
    const resize = screen.getByRole("button", { name: "调整侧边栏宽度" });
    fireEvent.pointerDown(resize, { clientX: 256 });
    fireEvent.pointerMove(window, { clientX: 356 });
    fireEvent.pointerUp(window);
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

  it("restores the sidebar and document layout from local storage", async () => {
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

    expect(document.querySelector(".reader-canvas-wide")).toBeTruthy();
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
          listFiles: vi.fn().mockResolvedValue({ files: [], defaultPath: "" }),
        })}
      />,
    );
    expect(await screen.findAllByText("目录中没有 Markdown 文件")).toHaveLength(
      2,
    );
    expect(window.location.pathname + window.location.search).toBe(
      "/?mode=auto",
    );
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
      return { path, title: file.title, html: `<p>Body for ${path}</p>` };
    }),
    ...overrides,
  };
}
