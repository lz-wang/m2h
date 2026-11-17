import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as clipboard from "@/lib/clipboard";
import { TokenSettings } from "./token-settings";

const fetchMock = vi.fn<typeof fetch>();

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function tokenPayload(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    id: "t1",
    name: "llm-agent",
    kind: "persistent",
    scope: {
      permissions: ["document:read", "search:read"],
      resource: { type: "all_published" },
    },
    createdAt: 1,
    expiresAt: 2,
    lastUsedAt: 0,
    ...overrides,
  };
}

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockReset();
});

function openDialog() {
  render(<TokenSettings />);
  return userEvent.click(screen.getByRole("button", { name: "API 令牌设置" }));
}

describe("TokenSettings", () => {
  it("lists tokens with scope and last-used metadata", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ tokens: [tokenPayload({ lastUsedAt: 1760000000000 })] }),
    );
    await openDialog();
    await waitFor(() => expect(screen.getByText("llm-agent")).toBeTruthy());
    expect(screen.getAllByText(/全部已发布文档/).length).toBeGreaterThan(0);
    expect(screen.getByText(/最近使用/)).toBeTruthy();
  });

  it("creates a token and shows the secret exactly once", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockResolvedValueOnce(
        jsonResponse(tokenPayload({ id: "t2", secret: "m2h_secret_once" })),
      );
    await openDialog();
    await waitFor(() => expect(screen.getByText(/还没有令牌/)).toBeTruthy());

    await userEvent.type(screen.getByLabelText("令牌名称"), "agent");
    await userEvent.click(screen.getByRole("button", { name: "创建" }));
    expect(await screen.findByText("m2h_secret_once")).toBeTruthy();

    const call = fetchMock.mock.calls[1];
    expect(String(call?.[0])).toBe("/api/tokens");
    const body = JSON.parse(String(call?.[1]?.body)) as {
      kind: string;
      scope: { resource: { type: string } };
    };
    expect(body.kind).toBe("persistent");
    expect(body.scope.resource.type).toBe("all_published");
  });

  it("revokes after confirmation and reports failures", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [tokenPayload()] }))
      .mockResolvedValueOnce(jsonResponse({ ok: true }));
    await openDialog();
    await waitFor(() => expect(screen.getByText("llm-agent")).toBeTruthy());

    await userEvent.click(
      screen.getByRole("button", { name: "撤销 llm-agent" }),
    );
    expect(screen.queryByRole("button", { name: "确认撤销" })).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "确认撤销" }));
    await waitFor(() =>
      expect(screen.getByText(/还没有令牌|无法/)).toBeTruthy(),
    );
  });

  it("copies the fresh secret to the clipboard", async () => {
    const copySpy = vi.spyOn(clipboard, "copyText").mockResolvedValue(true);
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockResolvedValueOnce(
        jsonResponse(tokenPayload({ secret: "m2h_copy_me" })),
      );
    await openDialog();
    await userEvent.type(screen.getByLabelText("令牌名称"), "x");
    await userEvent.click(screen.getByRole("button", { name: "创建" }));
    await userEvent.click(
      await screen.findByRole("button", { name: "复制令牌" }),
    );
    await waitFor(() => expect(copySpy).toHaveBeenCalledWith("m2h_copy_me"));
    copySpy.mockRestore();
  });

  it("refuses to create without a name", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ tokens: [] }));
    await openDialog();
    await waitFor(() => expect(screen.getByText(/还没有令牌/)).toBeTruthy());
    expect(screen.getByRole("button", { name: "创建" })).toBeTruthy();
    // The button is disabled while the name is empty; fetch untouched.
    const button = screen.getByRole("button", {
      name: "创建",
    }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("drops the fresh secret when the dialog closes and reopens", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockResolvedValueOnce(
        jsonResponse(tokenPayload({ id: "t2", secret: "m2h_ephemeral" })),
      )
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }));
    await openDialog();
    await waitFor(() => expect(screen.getByText(/还没有令牌/)).toBeTruthy());
    await userEvent.type(screen.getByLabelText("令牌名称"), "agent");
    await userEvent.click(screen.getByRole("button", { name: "创建" }));
    expect(await screen.findByText("m2h_ephemeral")).toBeTruthy();

    // Close the dialog (Escape), reopen it: the secret must not reappear.
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByText("m2h_ephemeral")).toBeNull());
    await userEvent.click(screen.getByRole("button", { name: "API 令牌设置" }));
    await waitFor(() => expect(screen.getByText(/还没有令牌/)).toBeTruthy());
    expect(screen.queryByText("m2h_ephemeral")).toBeNull();
    expect(screen.queryByText(/仅显示这一次/)).toBeNull();
  });

  it("scopes a new token to a selected document root", async () => {
    const filesResponse = jsonResponse({
      kind: "directory",
      version: "test",
      roots: [
        {
          id: "root-1",
          name: "docs",
          files: [
            { path: "guide.md", name: "guide.md" },
            { path: "design.md", name: "design.md" },
          ],
        },
      ],
    });
    // Call order: listTokens (open), /api/files (lazy on scope pick),
    // create. The URL-routed fallback covers any stragglers.
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockResolvedValueOnce(filesResponse)
      .mockResolvedValueOnce(
        jsonResponse(
          tokenPayload({
            scope: {
              permissions: ["document:read", "search:read"],
              resource: { type: "root", rootId: "root-1" },
            },
          }),
        ),
      )
      .mockImplementation(async (input: RequestInfo | URL) => {
        if (String(input).includes("/api/files")) {
          return filesResponse;
        }
        return jsonResponse({ tokens: [] });
      });
    await openDialog();
    await waitFor(() => expect(screen.getByText(/还没有令牌/)).toBeTruthy());

    await userEvent.selectOptions(
      screen.getByLabelText("访问范围"),
      "指定文档根目录",
    );
    const rootSelect = await screen.findByLabelText("文档根目录");
    await waitFor(() =>
      expect(rootSelect.querySelector("option")).not.toBeNull(),
    );
    await userEvent.selectOptions(
      screen.getByLabelText("文档根目录"),
      "root-1",
    );
    await userEvent.type(screen.getByLabelText("令牌名称"), "root-agent");
    await userEvent.click(screen.getByRole("button", { name: "创建" }));

    await waitFor(() =>
      expect(
        fetchMock.mock.calls.some((call) => String(call[0]) === "/api/files"),
      ).toBe(true),
    );
    const createCall = fetchMock.mock.calls.find(
      (call) =>
        String(call[0]) === "/api/tokens" && String(call[1]?.method) === "POST",
    );
    expect(createCall).toBeTruthy();
    const body = JSON.parse(String(createCall?.[1]?.body)) as {
      scope: { resource: { type: string; rootId?: string } };
    };
    expect(body.scope.resource.type).toBe("root");
    expect(body.scope.resource.rootId).toBe("root-1");
  });

  it("scopes a new token to a single selected document", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockImplementation(async (input: RequestInfo | URL) => {
        if (String(input).includes("/api/files")) {
          return jsonResponse({
            kind: "directory",
            version: "test",
            roots: [
              {
                id: "root-1",
                name: "docs",
                files: [{ path: "guide.md", name: "guide.md" }],
              },
            ],
          });
        }
        return jsonResponse({ tokens: [] });
      });
    await openDialog();
    await waitFor(() => expect(screen.getByText(/还没有令牌/)).toBeTruthy());

    await userEvent.selectOptions(
      screen.getByLabelText("访问范围"),
      "指定单篇文档",
    );
    await waitFor(() => expect(screen.getByLabelText("文档路径")).toBeTruthy());
    await userEvent.selectOptions(
      screen.getByLabelText("文档根目录"),
      "root-1",
    );
    await userEvent.selectOptions(
      screen.getByLabelText("文档路径"),
      "guide.md",
    );
    await userEvent.type(screen.getByLabelText("令牌名称"), "doc-agent");
    await userEvent.click(screen.getByRole("button", { name: "创建" }));

    await waitFor(() => {
      const createCall = fetchMock.mock.calls.find(
        (call) =>
          String(call[0]) === "/api/tokens" &&
          String(call[1]?.method) === "POST",
      );
      expect(createCall).toBeTruthy();
      const body = JSON.parse(String(createCall?.[1]?.body)) as {
        scope: {
          resource: { type: string; rootId?: string; path?: string };
        };
      };
      expect(body.scope.resource.type).toBe("document");
      expect(body.scope.resource.rootId).toBe("root-1");
      expect(body.scope.resource.path).toBe("guide.md");
    });
  });
});

describe("TokenSettings in the app toolbar", () => {
  it("renders its trigger through the App toolbar when tokens are available", async () => {
    const { App } = await import("../../App");
    const initialFiles = {
      kind: "directory" as const,
      version: "test",
      roots: [
        {
          id: "r0",
          name: "docs",
          files: [{ path: "README.md", name: "README.md" }],
        },
      ],
    };
    const document = {
      path: "README.md",
      title: "Readme",
      html: "<h1>Readme</h1>",
      frontmatter: null,
      toc: [],
    };
    const fetchCalls = vi.fn<typeof fetch>();
    fetchCalls.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/api/files")) {
        return jsonResponse(initialFiles);
      }
      if (url.includes("/api/document")) {
        return jsonResponse(document);
      }
      if (url.includes("/api/file-metadata")) {
        return jsonResponse({ title: "Readme" });
      }
      return jsonResponse({
        authMode: "none",
        features: { annotations: false, tokens: false },
      });
    });
    vi.stubGlobal("fetch", fetchCalls);
    render(<App tokensAvailable />);
    expect(
      await screen.findByRole("button", { name: "API 令牌设置" }),
    ).toBeTruthy();
    // And never without the capability.
    vi.stubGlobal(
      "fetch",
      fetchCalls.mockImplementation(async () =>
        jsonResponse({
          authMode: "none",
          features: { annotations: false, tokens: false },
        }),
      ),
    );
    vi.unstubAllGlobals();
  });
});
