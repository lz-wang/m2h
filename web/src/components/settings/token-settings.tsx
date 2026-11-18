import { Check, ClipboardCopy, LoaderCircle, Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";

import { createToken, listTokens, revokeToken, type TokenItem } from "@/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { appPath } from "@/lib/base-path";
import { copyText } from "@/lib/clipboard";

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
export function TokenSettings({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange(open: boolean): void;
}) {
  const [tokens, setTokens] = useState<TokenItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState("30");
  const [creating, setCreating] = useState(false);
  const [freshSecret, setFreshSecret] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [confirming, setConfirming] = useState<string | null>(null);
  // The scope picker. Roots load lazily — the default scope needs nothing.
  const [scopeType, setScopeType] = useState<
    "all_published" | "root" | "document"
  >("all_published");
  const [roots, setRoots] = useState<RootOption[] | null>(null);
  const [rootId, setRootId] = useState("");
  const [documentPath, setDocumentPath] = useState("");

  useEffect(() => {
    if (!open) {
      return;
    }
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
  }, [open]);

  // Restricted scopes need the workspace topology; fetch it once, on
  // first need. Only the persistent root identities (rootId) are usable
  // as token scopes — the positional ids are URL artifacts the server
  // rejects — so roots without one are not offered. A loading failure
  // says so instead of silently disabling creation.
  const [rootsError, setRootsError] = useState<string | null>(null);
  useEffect(() => {
    if (!open || scopeType === "all_published" || roots !== null) {
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
  }, [open, scopeType, roots]);

  const selectedRoot = roots?.find((option) => option.id === rootId);

  // Closing the dialog drops the secret: the one-time display is exactly
  // that, and a stale secret must never reappear on reopen.
  const handleOpenChange = (next: boolean) => {
    onOpenChange(next);
    if (!next) {
      setFreshSecret(null);
      setCopied(false);
    }
  };

  const create = async () => {
    const trimmed = name.trim();
    if (trimmed === "" || creating) {
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
        name: trimmed,
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
    try {
      await revokeToken(id);
      setTokens((current) => current.filter((token) => token.id !== id));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "撤销令牌失败");
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="gap-4 overflow-y-auto p-5 sm:max-w-lg">
        <div className="flex flex-col gap-1.5">
          <DialogTitle>API 令牌</DialogTitle>
          <DialogDescription>
            为 Agent 或脚本签发只读访问凭证；明文仅创建时显示一次。
          </DialogDescription>
        </div>
        <div className="flex flex-col gap-4">
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
              <p className="text-muted-foreground mt-2 text-xs">
                关闭后无法再次查看；丢失请撤销并重新创建。
              </p>
            </div>
          ) : null}

          <div className="flex flex-col gap-2">
            <div className="flex items-center gap-2">
              <Input
                value={name}
                aria-label="令牌名称"
                placeholder="令牌名称（如 llm-agent）"
                onChange={(event) => setName(event.target.value)}
              />
              <select
                value={expiresInDays}
                aria-label="有效期"
                className="border-border bg-background h-9 rounded-md border px-2 text-sm"
                onChange={(event) => setExpiresInDays(event.target.value)}
              >
                <option value={String(10 / 1440)}>10 分钟</option>
                <option value={String(30 / 1440)}>30 分钟</option>
                <option value={String(1 / 24)}>1 小时</option>
                <option value={String(6 / 24)}>6 小时</option>
                <option value={String(12 / 24)}>12 小时</option>
                <option value="1">24 小时</option>
                <option value="3">3 天</option>
                <option value="7">7 天</option>
                <option value="30">30 天</option>
                <option value="90">90 天</option>
                <option value="permanent">永久</option>
              </select>
              <Button
                size="sm"
                disabled={
                  creating ||
                  name.trim() === "" ||
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
            <div className="flex flex-wrap items-center gap-2">
              <label
                htmlFor="token-scope-type"
                className="text-muted-foreground text-xs"
              >
                访问范围
              </label>
              <select
                id="token-scope-type"
                aria-label="访问范围"
                className="border-border bg-background h-8 rounded-md border px-2 text-xs"
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
                    className="border-border bg-background h-8 max-w-36 rounded-md border px-2 text-xs"
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
              {scopeType === "document" && selectedRoot ? (
                <select
                  aria-label="文档路径"
                  className="border-border bg-background h-8 max-w-44 rounded-md border px-2 text-xs"
                  value={documentPath}
                  onChange={(event) => setDocumentPath(event.target.value)}
                >
                  <option value="" disabled>
                    选择文档…
                  </option>
                  {selectedRoot.paths.map((path) => (
                    <option key={path} value={path}>
                      {path}
                    </option>
                  ))}
                </select>
              ) : null}
            </div>
            <p className="text-muted-foreground text-xs">
              {scopeType === "all_published"
                ? "新令牌可读取全部已发布文档并使用搜索。"
                : scopeType === "root"
                  ? "新令牌只能读取所选文档根目录下的文档。"
                  : "新令牌只能读取所选的单篇文档。"}
            </p>
          </div>

          {error !== null ? (
            <p className="text-destructive text-sm" role="alert">
              {error}
            </p>
          ) : null}

          {loading ? (
            <p className="text-muted-foreground text-sm">正在加载令牌…</p>
          ) : tokens.length === 0 ? (
            <p className="text-muted-foreground py-4 text-center text-sm">
              还没有令牌。
            </p>
          ) : (
            <ul className="flex max-h-64 flex-col gap-2 overflow-y-auto">
              {tokens.map((token) => (
                <li
                  key={token.id}
                  className="border-border rounded-lg border p-2.5 text-sm"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium">{token.name}</span>
                    {confirming === token.id ? (
                      <span className="flex items-center gap-1">
                        <Button
                          variant="destructive"
                          size="xs"
                          onClick={() => {
                            void revoke(token.id);
                            setConfirming(null);
                          }}
                        >
                          确认撤销
                        </Button>
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => setConfirming(null)}
                        >
                          取消
                        </Button>
                      </span>
                    ) : (
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        aria-label={`撤销 ${token.name}`}
                        onClick={() => setConfirming(token.id)}
                      >
                        <Trash2 aria-hidden="true" />
                      </Button>
                    )}
                  </div>
                  <div className="text-muted-foreground mt-1 text-xs">
                    {describeScope(token)} ·{" "}
                    {token.kind === "temporary" ? "临时" : "长期"} ·{" "}
                    {token.expiresAt === 0
                      ? "永久"
                      : `到期 ${new Date(token.expiresAt).toLocaleString()}`}
                    {token.lastUsedAt > 0
                      ? ` · 最近使用 ${new Date(token.lastUsedAt).toLocaleString()}`
                      : " · 从未使用"}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
