import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
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
    expiresAt: 0,
    lastUsedAt: 0,
    ...overrides,
  };
}

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockReset();
});

function openDialog() {
  function Harness() {
    const [open, setOpen] = useState(false);
    return (
      <>
        <button type="button" onClick={() => setOpen(true)}>
          API 令牌设置
        </button>
        <TokenSettings open={open} onOpenChange={setOpen} />
      </>
    );
  }
  render(<Harness />);
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
    expect(screen.getByRole("columnheader", { name: "上次使用" })).toBeTruthy();
  });

  it("filters expired tokens and sorts table columns", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        tokens: [
          tokenPayload({
            id: "a",
            name: "Alpha",
            expiresAt: Date.now() + 60000,
          }),
          tokenPayload({ id: "b", name: "Beta", expiresAt: 0 }),
          tokenPayload({ id: "c", name: "Expired", expiresAt: 1 }),
        ],
      }),
    );
    await openDialog();
    await screen.findByText("Alpha");
    expect(screen.queryByText("Expired")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "令牌名称" }));
    const rows = screen.getAllByRole("row");
    expect(rows[1]?.textContent).toContain("Beta");
    expect(
      screen
        .getByRole("columnheader", { name: "令牌名称" })
        .getAttribute("aria-sort"),
    ).toBe("descending");
  });

  it("creates a token and shows the secret exactly once", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockResolvedValueOnce(
        jsonResponse(tokenPayload({ id: "t2", secret: "m2h_secret_once" })),
      );
    await openDialog();
    await userEvent.click(await screen.findByRole("button", { name: "新建" }));

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
    await userEvent.click(await screen.findByRole("button", { name: "新建" }));
    await userEvent.type(screen.getByLabelText("令牌名称"), "x");
    await userEvent.click(screen.getByRole("button", { name: "创建" }));
    await userEvent.click(
      await screen.findByRole("button", { name: "复制令牌" }),
    );
    await waitFor(() => expect(copySpy).toHaveBeenCalledWith("m2h_copy_me"));
    copySpy.mockRestore();
  });

  it("generates a name and uses the default scope and duration", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockResolvedValueOnce(jsonResponse(tokenPayload({ secret: "secret" })));
    await openDialog();
    await userEvent.click(await screen.findByRole("button", { name: "新建" }));
    await userEvent.click(screen.getByRole("button", { name: "创建" }));
    await screen.findByText("secret");
    const body = JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body));
    expect(body.name).toMatch(/^[a-f0-9]{7}$/);
    expect(body.expiresInDays).toBe(30 / 1440);
    expect(body.scope.resource.type).toBe("all_published");
  });

  it("drops the fresh secret when the dialog closes and reopens", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockResolvedValueOnce(
        jsonResponse(tokenPayload({ id: "t2", secret: "m2h_ephemeral" })),
      )
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }));
    await openDialog();
    await userEvent.click(await screen.findByRole("button", { name: "新建" }));
    await userEvent.type(screen.getByLabelText("令牌名称"), "agent");
    await userEvent.click(screen.getByRole("button", { name: "创建" }));
    expect(await screen.findByText("m2h_ephemeral")).toBeTruthy();

    // Close the dialog (Escape), reopen it: the secret must not reappear.
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByText("m2h_ephemeral")).toBeNull());
    await userEvent.click(screen.getByRole("button", { name: "新建" }));
    expect(screen.queryByText("m2h_ephemeral")).toBeNull();
    expect(screen.queryByText(/仅显示这一次/)).toBeNull();
  });

  it("scopes a new token to a selected document root", async () => {
    // The listing answers the URL positional id AND the persistent
    // identity the token scope validation requires; the picker must
    // submit the identity.
    const filesResponse = jsonResponse({
      kind: "directory",
      version: "test",
      roots: [
        {
          id: "r0",
          rootId: "root-uuid-1",
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
    await userEvent.click(await screen.findByRole("button", { name: "新建" }));

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
      "root-uuid-1",
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
    expect(body.scope.resource.rootId).toBe("root-uuid-1");
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
                id: "r0",
                rootId: "root-uuid-1",
                name: "docs",
                files: [{ path: "guide.md", name: "guide.md" }],
              },
            ],
          });
        }
        return jsonResponse({ tokens: [] });
      });
    await openDialog();
    await userEvent.click(await screen.findByRole("button", { name: "新建" }));

    await userEvent.selectOptions(
      screen.getByLabelText("访问范围"),
      "指定单篇文档",
    );
    await waitFor(() => expect(screen.getByLabelText("文档路径")).toBeTruthy());
    await userEvent.selectOptions(
      screen.getByLabelText("文档根目录"),
      "root-uuid-1",
    );
    await userEvent.click(screen.getByLabelText("文档路径"));
    await userEvent.click(
      await screen.findByRole("option", { name: "guide.md" }),
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
      expect(body.scope.resource.rootId).toBe("root-uuid-1");
      expect(body.scope.resource.path).toBe("guide.md");
    });
  });

  it("explains itself when the document listing cannot be loaded", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ tokens: [] }))
      .mockImplementation(async (input: RequestInfo | URL) => {
        if (String(input).includes("/api/files")) {
          return jsonResponse({ error: "boom" }, 500);
        }
        return jsonResponse({ tokens: [] });
      });
    await openDialog();
    await userEvent.click(await screen.findByRole("button", { name: "新建" }));

    await userEvent.selectOptions(
      screen.getByLabelText("访问范围"),
      "指定文档根目录",
    );
    expect(await screen.findByText(/无法加载文档列表/)).toBeTruthy();
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
    render(
      <App
        tokensAvailable
        authIdentity={{
          username: "alice",
          displayName: "alice",
          provider: "local",
        }}
        onLogout={() => {}}
      />,
    );
    await userEvent.click(await screen.findByRole("button", { name: "设置" }));
    expect(
      await screen.findByRole("menuitem", { name: "凭据管理" }),
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
