import { Select } from "@base-ui/react/select";
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  Check,
  ChevronDown,
  ClipboardCopy,
  LoaderCircle,
  Plus,
  Trash2,
} from "lucide-react";
import { useEffect, useState } from "react";
import { createToken, listTokens, revokeToken, type TokenItem } from "@/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { appPath } from "@/lib/base-path";
import { copyText } from "@/lib/clipboard";
import { expiryOptions } from "../../lib/expiry";

function describeScope(token: TokenItem): string {
  switch (token.scope.resource.type) {
    case "all_published":
      return "全部已发布文档";
    case "root":
      // The identity travels with the token; it is what a multi-root
      // workspace can tell two root-scoped tokens apart by.
      return `单个文档根（${token.scope.resource.rootId ?? "?"}）`;
    case "document":
      return `单篇文档（${token.scope.resource.path ?? ""}）`;
    default:
      return "未知范围";
  }
}

// One selectable document root from /api/files.
interface RootOption {
  id: string;
  name: string;
  paths: string[];
}

// TokenSettings is the authenticated deployment's API-token manager:
// create with a scope picker (secret shown exactly once), list with
// last-used time, revoke. Scope edits are deliberately absent — a token
// whose reach changed would need re-issuing; revocation plus a new token
// is the honest flow.
export function TokenSettings() {
  const [tokens, setTokens] = useState<TokenItem[]>([]);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const nextExpiry = Math.min(
      ...tokens
        .map((token) => token.expiresAt)
        .filter((expiry) => expiry > now),
    );
    if (!Number.isFinite(nextExpiry)) return;
    const timer = window.setTimeout(
      () => setNow(Date.now()),
      Math.min(nextExpiry - now + 1, 2_147_483_647),
    );
    return () => window.clearTimeout(timer);
  }, [tokens, now]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState(String(30 / 1440));
  const [createOpen, setCreateOpen] = useState(false);
  const [sort, setSort] = useState<{
    key: "name" | "scope" | "expiresAt" | "lastUsedAt";
    ascending: boolean;
  }>({ key: "lastUsedAt", ascending: false });
  const [creating, setCreating] = useState(false);
  const [freshSecret, setFreshSecret] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState<string | null>(null);
  // The scope picker. Roots load lazily — the default scope needs nothing.
  const [scopeType, setScopeType] = useState<
    "all_published" | "root" | "document"
  >("all_published");
  const [roots, setRoots] = useState<RootOption[] | null>(null);
  const [rootId, setRootId] = useState("");
  const [documentPath, setDocumentPath] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    listTokens(controller.signal)
      .then((items) => {
        if (!controller.signal.aborted) {
          setTokens(items);
        }
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          setError(cause instanceof Error ? cause.message : "无法加载令牌");
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });
    return () => {
      controller.abort();
    };
  }, []);

  // Restricted scopes need the workspace topology; fetch it once, on
  // first need. Only the persistent root identities (rootId) are usable
  // as token scopes — the positional ids are URL artifacts the server
  // rejects — so roots without one are not offered. A loading failure
  // says so instead of silently disabling creation.
  const [rootsError, setRootsError] = useState<string | null>(null);
  useEffect(() => {
    if (scopeType === "all_published" || roots !== null) {
      return;
    }
    const controller = new AbortController();
    setRootsError(null);
    void fetch(appPath("/api/files"), { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error("无法加载文档列表");
        }
        return (await response.json()) as {
          roots?: Array<{
            rootId?: string;
            name?: string;
            files?: Array<{ path?: string }>;
          }>;
        };
      })
      .then((payload) => {
        if (controller.signal.aborted) {
          return;
        }
        const options = (payload.roots ?? [])
          .filter(
            (
              root,
            ): root is {
              rootId: string;
              name: string;
              files: Array<{ path: string }>;
            } =>
              typeof root.rootId === "string" &&
              root.rootId !== "" &&
              typeof root.name === "string" &&
              Array.isArray(root.files),
          )
          .map((root) => ({
            id: root.rootId,
            name: root.name,
            paths: root.files
              .map((file) => file.path)
              .filter((path): path is string => typeof path === "string"),
          }));
        setRoots(options);
        if (options.length > 0) {
          setRootId((current) =>
            current === "" || options.every((option) => option.id !== current)
              ? options[0].id
              : current,
          );
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setRoots([]);
          setRootsError("无法加载文档列表，请稍后重试。");
        }
      });
    return () => {
      controller.abort();
    };
  }, [scopeType, roots]);

  const selectedRoot = roots?.find((option) => option.id === rootId);

  const create = async () => {
    const trimmed = name.trim();
    if (creating) {
      return;
    }
    if (scopeType !== "all_published" && rootId === "") {
      return;
    }
    if (scopeType === "document" && documentPath === "") {
      return;
    }
    setCreating(true);
    setError(null);
    try {
      const days = Number.parseFloat(expiresInDays);
      const created = await createToken({
        kind: "persistent",
        name:
          trimmed ||
          Array.from(crypto.getRandomValues(new Uint8Array(4)), (byte) =>
            byte.toString(16).padStart(2, "0"),
          )
            .join("")
            .slice(0, 7),
        ...(expiresInDays === "permanent"
          ? { permanent: true }
          : { expiresInDays: days }),
        scope: {
          permissions: ["document:read", "search:read"],
          resource:
            scopeType === "all_published"
              ? { type: "all_published" }
              : scopeType === "root"
                ? { type: "root", rootId }
                : { type: "document", rootId, path: documentPath },
        },
      });
      setTokens((current) => [created, ...current]);
      setFreshSecret(created.secret ?? null);
      setCopied(false);
      setName("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "创建令牌失败");
    } finally {
      setCreating(false);
    }
  };

  const revoke = async (id: string) => {
    if (revoking !== null) return;
    setRevoking(id);
    setError(null);
    try {
      await revokeToken(id);
      setTokens((current) => current.filter((token) => token.id !== id));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "删除令牌失败");
    } finally {
      setRevoking(null);
    }
  };

  const visibleTokens = tokens
    .filter((token) => token.expiresAt === 0 || token.expiresAt > now)
    .sort((a, b) => {
      const value = (token: TokenItem) =>
        sort.key === "scope" ? describeScope(token) : token[sort.key];
      const left = value(a);
      const right = value(b);
      const comparison =
        typeof left === "number" && typeof right === "number"
          ? sort.key === "expiresAt"
            ? (left || Infinity) - (right || Infinity)
            : left - right
          : String(left).localeCompare(String(right), "zh-CN");
      return (
        (Number.isNaN(comparison) ? 0 : comparison) * (sort.ascending ? 1 : -1)
      );
    });
  const headers = [
    ["name", "令牌名称"],
    ["scope", "访问范围"],
    ["expiresAt", "到期时间"],
    ["lastUsedAt", "上次使用"],
  ] as const;

  return (
    <>
      <div
        className={`flex items-center justify-between gap-4 pb-4${visibleTokens.length > 0 ? " border-b" : ""}`}
      >
        <h2 className="text-sm font-medium">API 令牌</h2>
        <Button
          size="sm"
          onClick={() => {
            setName("");
            setExpiresInDays(String(30 / 1440));
            setScopeType("all_published");
            setDocumentPath("");
            setFreshSecret(null);
            setError(null);
            setCreateOpen(true);
          }}
        >
          <Plus aria-hidden="true" />
          新建
        </Button>
      </div>
      {error !== null && !createOpen ? (
        <p className="text-destructive text-sm" role="alert">
          {error}
        </p>
      ) : null}
      <div className="min-h-0 flex-1 overflow-auto">
        {loading ? (
          <p className="text-muted-foreground py-6 text-sm">正在加载令牌…</p>
        ) : visibleTokens.length === 0 ? (
          <p className="text-muted-foreground py-10 text-center text-sm">
            还没有令牌
          </p>
        ) : (
          <table className="w-full min-w-[700px] text-left text-sm">
            <thead className="bg-popover sticky top-0 text-muted-foreground">
              <tr>
                {headers.map(([key, label]) => (
                  <th
                    key={key}
                    scope="col"
                    aria-sort={
                      sort.key === key
                        ? sort.ascending
                          ? "ascending"
                          : "descending"
                        : "none"
                    }
                    className="border-b px-3 py-3 font-medium"
                  >
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 hover:text-foreground"
                      onClick={() =>
                        setSort((current) => ({
                          key,
                          ascending:
                            current.key === key ? !current.ascending : true,
                        }))
                      }
                    >
                      {label}
                      {sort.key === key ? (
                        sort.ascending ? (
                          <ArrowUp className="size-3.5" />
                        ) : (
                          <ArrowDown className="size-3.5" />
                        )
                      ) : (
                        <ArrowUpDown className="size-3.5" />
                      )}
                    </button>
                  </th>
                ))}
                <th scope="col" className="border-b px-3 py-3 font-medium">
                  操作
                </th>
              </tr>
            </thead>
            <tbody>
              {visibleTokens.map((token) => (
                <tr
                  key={token.id}
                  className="border-b last:border-0 hover:bg-muted/40"
                >
                  <td className="px-3 py-3 font-medium">{token.name}</td>
                  <td className="px-3 py-3">{describeScope(token)}</td>
                  <td className="px-3 py-3 whitespace-nowrap">
                    {token.expiresAt === 0
                      ? "永久"
                      : new Date(token.expiresAt).toLocaleString()}
                  </td>
                  <td className="px-3 py-3 whitespace-nowrap">
                    {token.lastUsedAt > 0
                      ? new Date(token.lastUsedAt).toLocaleString()
                      : "从未使用"}
                  </td>
                  <td className="px-3 py-3">
                    <Button
                      variant="ghost"
                      size="icon-xs"
                      aria-label={`删除 ${token.name}`}
                      disabled={revoking !== null}
                      onClick={() => void revoke(token.id)}
                    >
                      <Trash2 aria-hidden="true" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <Dialog
        open={createOpen}
        onOpenChange={(next) => {
          setCreateOpen(next);
          if (!next) setCopied(false);
        }}
        onOpenChangeComplete={(next) => {
          if (!next) setFreshSecret(null);
        }}
      >
        <DialogContent
          aria-describedby={undefined}
          forceBackdrop
          overlayClassName="z-[60] bg-black/30 supports-backdrop-filter:backdrop-blur-sm"
          className="top-1/2 z-[61] -translate-y-1/2 gap-4 p-5 data-ending-style:-translate-y-[calc(50%-0.5rem)] data-starting-style:-translate-y-[calc(50%-0.5rem)] sm:max-w-lg"
        >
          <DialogTitle>新建 API 令牌</DialogTitle>
          {freshSecret !== null ? (
            <div className="bg-muted/50 rounded-lg border p-3">
              <p className="mb-2 text-sm font-medium">
                令牌已创建，明文仅显示这一次：
              </p>
              <div className="flex items-center gap-2">
                <code className="min-w-0 flex-1 overflow-x-auto rounded bg-background px-2 py-1 text-xs whitespace-nowrap">
                  {freshSecret}
                </code>
                <Button
                  variant="outline"
                  size="icon-xs"
                  aria-label="复制令牌"
                  onClick={() => {
                    void copyText(freshSecret).then((ok) => {
                      setCopied(ok);
                      window.setTimeout(() => setCopied(false), 1500);
                    });
                  }}
                >
                  {copied ? (
                    <Check aria-hidden="true" />
                  ) : (
                    <ClipboardCopy aria-hidden="true" />
                  )}
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              <label
                htmlFor="token-name"
                className="flex flex-col gap-1.5 text-sm"
              >
                令牌名称（可选）
                <Input
                  value={name}
                  id="token-name"
                  aria-label="令牌名称"
                  placeholder="留空自动生成"
                  onChange={(event) => setName(event.target.value)}
                />
              </label>
              <label className="flex flex-col gap-1.5 text-sm">
                有效期
                <select
                  value={expiresInDays}
                  aria-label="有效期"
                  className="border-border bg-background h-9 rounded-md border px-2 text-sm"
                  onChange={(event) => setExpiresInDays(event.target.value)}
                >
                  {expiryOptions.map(({ id, days, label }) => (
                    <option key={id} value={String(days)}>
                      {label}
                    </option>
                  ))}
                </select>
              </label>
              <div className="flex flex-col gap-1.5 text-sm">
                <label htmlFor="token-scope-type">访问范围</label>
                <div className="flex gap-2">
                  <select
                    id="token-scope-type"
                    aria-label="访问范围"
                    className="border-border bg-background h-9 min-w-0 flex-1 rounded-md border px-2 text-sm"
                    value={
                      scopeType === "all_published"
                        ? "全部已发布文档"
                        : scopeType === "root"
                          ? "指定文档根目录"
                          : "指定单篇文档"
                    }
                    onChange={(event) => {
                      const next = event.target.value;
                      setScopeType(
                        next === "指定文档根目录"
                          ? "root"
                          : next === "指定单篇文档"
                            ? "document"
                            : "all_published",
                      );
                    }}
                  >
                    <option value="全部已发布文档">全部已发布文档</option>
                    <option value="指定文档根目录">指定文档根目录</option>
                    <option value="指定单篇文档">指定单篇文档</option>
                  </select>
                  {scopeType !== "all_published" ? (
                    rootsError !== null ? (
                      <span className="text-destructive inline-flex items-center gap-1 text-xs">
                        {rootsError}
                        <Button
                          variant="ghost"
                          size="xs"
                          aria-label="重试加载文档列表"
                          onClick={() => setRoots(null)}
                        >
                          重试
                        </Button>
                      </span>
                    ) : (
                      <select
                        aria-label="文档根目录"
                        className="border-border bg-background h-9 min-w-0 flex-1 rounded-md border px-2 text-sm"
                        value={rootId}
                        onChange={(event) => {
                          setRootId(event.target.value);
                          setDocumentPath("");
                        }}
                      >
                        {(roots ?? []).map((option) => (
                          <option key={option.id} value={option.id}>
                            {option.name}
                          </option>
                        ))}
                      </select>
                    )
                  ) : null}
                </div>
                {scopeType === "document" && selectedRoot ? (
                  <Select.Root
                    value={documentPath || null}
                    onValueChange={(value) => setDocumentPath(value ?? "")}
                  >
                    <Select.Trigger
                      aria-label="文档路径"
                      className="border-border bg-background flex h-9 w-full min-w-0 items-center justify-between gap-2 rounded-md border px-2 text-left text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <Select.Value
                        placeholder="选择文档…"
                        className="min-w-0 truncate"
                      />
                      <Select.Icon>
                        <ChevronDown className="size-4 shrink-0" />
                      </Select.Icon>
                    </Select.Trigger>
                    <Select.Portal>
                      <Select.Positioner
                        className="z-[70]"
                        sideOffset={4}
                        alignItemWithTrigger={false}
                      >
                        <Select.Popup className="border-border bg-popover text-popover-foreground max-h-60 w-[var(--anchor-width)] min-w-0 overflow-y-auto rounded-md border p-1 shadow-lg">
                          <Select.List>
                            {selectedRoot.paths.map((path) => (
                              <Select.Item
                                key={path}
                                value={path}
                                className="relative flex cursor-default items-center gap-2 rounded px-2 py-1.5 text-sm outline-none data-highlighted:bg-accent data-highlighted:text-accent-foreground"
                              >
                                <Select.ItemText className="min-w-0 flex-1 truncate">
                                  {path}
                                </Select.ItemText>
                                <Select.ItemIndicator>
                                  <Check className="size-4" />
                                </Select.ItemIndicator>
                              </Select.Item>
                            ))}
                          </Select.List>
                        </Select.Popup>
                      </Select.Positioner>
                    </Select.Portal>
                  </Select.Root>
                ) : null}
              </div>
              <Button
                className="self-end"
                size="sm"
                disabled={
                  creating ||
                  (scopeType !== "all_published" && rootId === "") ||
                  (scopeType === "document" && documentPath === "")
                }
                onClick={() => void create()}
              >
                {creating ? (
                  <LoaderCircle aria-hidden="true" className="animate-spin" />
                ) : (
                  <Plus aria-hidden="true" />
                )}
                创建
              </Button>
            </div>
          )}
          {error !== null ? (
            <p className="text-destructive text-sm" role="alert">
              {error}
            </p>
          ) : null}
        </DialogContent>
      </Dialog>
    </>
  );
}
