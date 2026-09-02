import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { APIError, type FileListResponse, type PreviewAPI } from "./api";
import {
  renderRichContent,
  rerenderThemeSensitiveContent,
} from "./lib/render-rich-content";

const renderRichContentMock = vi.mocked(renderRichContent);
const rerenderThemeSensitiveContentMock = vi.mocked(
  rerenderThemeSensitiveContent,
);

// The render-generation protocol between PreviewContent and the rich-content
// renderer, pinned at the component level. App.test.tsx exercises the real
// renderer; this file replaces it with controllable promises so the tests can
// park the initial enhancement exactly where the races live — a theme toggle
// while the initial render is still awaiting a runtime download, and rapid
// toggles whose repaints must serialize behind it.

vi.mock("./lib/render-rich-content", () => ({
  renderRichContent: vi.fn(),
  rerenderThemeSensitiveContent: vi.fn(),
  finalizeVegaLiteViews: vi.fn(),
}));

const initialFiles: FileListResponse = {
  kind: "directory",
  version: "0.9.1",
  roots: [
    {
      id: "r0",
      name: "docs",
      files: [
        { path: "README.md", name: "README.md", title: "Readme API Title" },
        { path: "guides/setup.md", name: "setup.md", title: "Setup API Title" },
      ],
    },
  ],
};

// One gated promise per renderRichContent call, resolved by the test at the
// exact moment the initial enhancement should finish.
const initialGates: Array<{ resolve(): void }> = [];

// The freshness token of the latest initial render, so a test can prove a
// theme toggle did not invalidate it.
let latestInitialIsCurrent: (() => boolean) | undefined;

beforeEach(() => {
  window.localStorage.clear();
  window.sessionStorage.clear();
  window.history.replaceState(null, "", "/doc/README.md");
  document.documentElement.className = "";
  delete document.documentElement.dataset.mode;
  vi.clearAllMocks();
  initialGates.length = 0;
  renderRichContentMock.mockImplementation((_root, _mode, isCurrent) => {
    latestInitialIsCurrent = isCurrent;
    return new Promise<void>((resolve) => {
      initialGates.push({ resolve });
    });
  });
  rerenderThemeSensitiveContentMock.mockResolvedValue(undefined);
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
    getMarkdown: vi.fn().mockResolvedValue("# raw"),
    ...overrides,
  };
}

async function openDocument(): Promise<void> {
  render(<App api={createAPI()} />);
  await screen.findByText("Body for README.md");
  expect(renderRichContentMock).toHaveBeenCalledTimes(1);
}

// Pick a theme through the toolbar menu. The button's accessible name changes
// with the active mode, so match the prefix. Base UI's non-modal menu stays
// open after a pick, and clicking the button again would only close it — so
// the item is looked up first and the menu opened only when it is closed.
async function pickTheme(
  user: ReturnType<typeof userEvent.setup>,
  item: string,
) {
  let option = screen.queryByRole("menuitemradio", { name: item });
  if (option === null) {
    await user.click(screen.getByRole("button", { name: /^显示主题：/ }));
    option = await screen.findByRole("menuitemradio", { name: item });
  }
  await user.click(option);
}

function resolveInitialGate(index: number): Promise<void> {
  return act(async () => {
    initialGates[index]?.resolve();
    await Promise.resolve();
  });
}

describe("theme repaint scheduling", () => {
  it("queues the repaint behind a pending initial render without invalidating it", async () => {
    const user = userEvent.setup();
    await openDocument();

    // Toggle while the initial enhancement is still parked on its gate — the
    // Vega runtime is "still downloading" and no chart container exists yet.
    await pickTheme(user, "深色");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(rerenderThemeSensitiveContentMock).not.toHaveBeenCalled();

    // The toggle must not retire the initial render: its freshness token is
    // scoped to body swaps only.
    expect(latestInitialIsCurrent?.()).toBe(true);

    await resolveInitialGate(0);
    await waitFor(() => {
      expect(rerenderThemeSensitiveContentMock).toHaveBeenCalledTimes(1);
    });
    expect(rerenderThemeSensitiveContentMock).toHaveBeenCalledWith(
      expect.any(HTMLElement),
      "dark",
      expect.any(Function),
    );
  });

  it("skips a superseded middle theme and keeps serving later toggles", async () => {
    const user = userEvent.setup();
    await openDocument();

    await pickTheme(user, "深色");
    await pickTheme(user, "浅色");
    await resolveInitialGate(0);

    // Both repaints were parked on the initial gate; by the time it opened,
    // the newest claimed theme was light and the body was last painted light,
    // so neither the dark nor the light repaint has work to do.
    await act(async () => {
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    });
    expect(rerenderThemeSensitiveContentMock).not.toHaveBeenCalled();

    // The queue is not wedged: a fresh toggle after the skipped pair still
    // repaints.
    await pickTheme(user, "深色");
    await waitFor(() => {
      expect(rerenderThemeSensitiveContentMock).toHaveBeenCalledWith(
        expect.any(HTMLElement),
        "dark",
        expect.any(Function),
      );
    });
  });

  it("runs a second repaint only after the first one settles", async () => {
    const user = userEvent.setup();
    render(<App api={createAPI()} />);
    await screen.findByText("Body for README.md");
    await resolveInitialGate(0);

    // Park the dark repaint mid-flight, then queue the light one behind it.
    let finishDark: () => void = () => {};
    rerenderThemeSensitiveContentMock.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          finishDark = resolve;
        }),
    );
    await pickTheme(user, "深色");
    await waitFor(() => {
      expect(rerenderThemeSensitiveContentMock).toHaveBeenCalledTimes(1);
    });

    await pickTheme(user, "浅色");
    await act(async () => {
      await Promise.resolve();
    });
    // Still serialized behind the pending dark repaint — never overlapped.
    expect(rerenderThemeSensitiveContentMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      finishDark();
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(rerenderThemeSensitiveContentMock).toHaveBeenCalledTimes(2);
    });
    expect(rerenderThemeSensitiveContentMock).toHaveBeenLastCalledWith(
      expect.any(HTMLElement),
      "light",
      expect.any(Function),
    );
  });

  it("retires queued repaints when the document body swaps", async () => {
    const user = userEvent.setup();
    await openDocument();

    await pickTheme(user, "深色");
    // The dark repaint is parked on the first body's initial gate.
    await user.click(screen.getByRole("button", { name: "guides" }));
    await user.click(
      screen.getByRole("button", { name: "Setup API Title，guides/setup.md" }),
    );
    await screen.findByText("Body for guides/setup.md");
    expect(renderRichContentMock).toHaveBeenCalledTimes(2);

    // Let the retired body's initial render settle: its queued repaint must
    // not fire against the new body.
    await resolveInitialGate(0);
    await act(async () => {
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    });
    expect(rerenderThemeSensitiveContentMock).not.toHaveBeenCalled();

    // The new body painted in the already-toggled dark theme, so nothing is
    // pending once its own initial render settles either.
    await resolveInitialGate(1);
    await act(async () => {
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    });
    expect(rerenderThemeSensitiveContentMock).not.toHaveBeenCalled();
  });
});
