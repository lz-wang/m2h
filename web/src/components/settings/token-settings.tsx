import {
  Check,
  ClipboardCopy,
  KeyRound,
  LoaderCircle,
  Plus,
  Trash2,
} from "lucide-react";
import { useEffect, useState } from "react";

import { createToken, listTokens, revokeToken, type TokenItem } from "@/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { copyText } from "@/lib/clipboard";

function describeScope(token: TokenItem): string {
  switch (token.scope.resource.type) {
    case "all_published":
      return "全部已发布文档";
    case "root":
      return "单个文档根";
    case "document":
      return `单篇文档（${token.scope.resource.path ?? ""}）`;
    default:
      return "未知范围";
  }
}

// TokenSettings is the authenticated deployment's API-token manager:
// create (secret shown exactly once), list with last-used time, revoke.
// Scope edits are deliberately absent — a token whose reach changed would
// need re-issuing; revocation plus a new token is the honest flow.
export function TokenSettings() {
  const [open, setOpen] = useState(false);
  const [tokens, setTokens] = useState<TokenItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState("180");
  const [creating, setCreating] = useState(false);
  const [freshSecret, setFreshSecret] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [confirming, setConfirming] = useState<string | null>(null);

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

  const create = async () => {
    const trimmed = name.trim();
    if (trimmed === "" || creating) {
      return;
    }
    setCreating(true);
    setError(null);
    try {
      const days = Number.parseFloat(expiresInDays);
      const created = await createToken({
        kind: "persistent",
        name: trimmed,
        expiresInDays: Number.isFinite(days) ? days : undefined,
        scope: {
          permissions: ["document:read", "search:read"],
          resource: { type: "all_published" },
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
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            className="toolbar-control"
            aria-label="API 令牌设置"
          >
            <KeyRound aria-hidden="true" />
          </Button>
        }
      />
      <DialogContent className="sm:max-w-lg">
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
              <Input
                value={expiresInDays}
                aria-label="有效天数"
                placeholder="180"
                className="w-24"
                inputMode="decimal"
                onChange={(event) => setExpiresInDays(event.target.value)}
              />
              <Button
                size="sm"
                disabled={creating || name.trim() === ""}
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
            <p className="text-muted-foreground text-xs">
              新令牌可读取全部已发布文档并使用搜索（有效期最长 365 天）。
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
                    {token.kind === "temporary" ? "临时" : "长期"} · 到期{" "}
                    {new Date(token.expiresAt).toLocaleDateString()}
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
