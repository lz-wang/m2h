import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  Check,
  ChevronDown,
  CircleAlert,
  Clock3,
  Copy,
  ExternalLink,
  FileText,
  FolderOpen,
  Link2,
  Trash2,
} from "lucide-react";
import { type ReactNode, useEffect, useMemo, useState } from "react";

import {
  APIError,
  createShare,
  type DocumentRef,
  type DocumentShare,
  listShares,
  revokeShare,
} from "../../api";
import { copyText } from "../../lib/clipboard";
import { expiryOptions } from "../../lib/expiry";
import { Button, buttonVariants } from "../ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "../ui/dialog";
import { Input } from "../ui/input";

export interface ShareTarget {
  documentRef: DocumentRef;
  title: string;
}

function useShareList(open: boolean, document?: DocumentRef) {
  const [shares, setShares] = useState<DocumentShare[] | null>(null);
  const [error, setError] = useState(false);
  const [revision, setRevision] = useState(0);
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    void revision;
    if (!open) {
      setShares(null);
      return;
    }
    const controller = new AbortController();
    setError(false);
    setNow(Date.now());
    void listShares(controller.signal, document).then(
      (items) => {
        if (!controller.signal.aborted) {
          setNow(Date.now());
          setShares(items);
        }
      },
      () => {
        if (!controller.signal.aborted) setError(true);
      },
    );
    return () => controller.abort();
  }, [open, revision, document]);
  useEffect(() => {
    if (!open || shares === null) return;
    const next = shares.reduce(
      (earliest, share) =>
        share.expiresAt > now ? Math.min(earliest, share.expiresAt) : earliest,
      Infinity,
    );
    if (!Number.isFinite(next)) return;
    // The expiry of a long-lived share may exceed the browser timer limit.
    const delay = Math.min(Math.max(next - Date.now(), 0), 2_147_483_647);
    const timer = window.setTimeout(() => setNow(Date.now()), delay);
    return () => window.clearTimeout(timer);
  }, [open, shares, now]);
  return {
    shares,
    setShares,
    error,
    now,
    retry: () => setRevision((value) => value + 1),
  };
}

function remaining(share: DocumentShare, now: number): string {
  if (share.expiresAt === 0) return "永久";
  const minutes = Math.floor((share.expiresAt - now) / 60_000);
  const hours = Math.floor(minutes / 60);
  if (hours >= 24) return `${Math.floor(hours / 24)}d${hours % 24}h`;
  if (hours > 0) return `${hours}h${minutes % 60}m`;
  return minutes > 0 ? `${minutes}m` : "不足 1 分钟";
}

function ShareExpiry({ share }: { share: DocumentShare }) {
  const { expiresAt } = share;
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const remainingTime = expiresAt - now;
    if (remainingTime <= 0) return;
    const unit = remainingTime >= 86_400_000 ? 3_600_000 : 60_000;
    // Only this label updates as the display unit changes. The list has its
    // own timer for the exact expiry, including the final minute.
    const next = Math.min(expiresAt, now + (remainingTime % unit) + 1);
    const timer = window.setTimeout(
      () => setNow(Date.now()),
      Math.max(next - Date.now(), 0),
    );
    return () => window.clearTimeout(timer);
  }, [expiresAt, now]);
  return (
    <span
      title={
        share.expiresAt === 0
          ? undefined
          : new Date(share.expiresAt).toLocaleString()
      }
    >
      {remaining(share, now)}
    </span>
  );
}

function ShareActions({
  share,
  onRevoke,
}: {
  share: DocumentShare;
  onRevoke(): void;
}) {
  return (
    <div className="flex items-center justify-center gap-1">
      <a
        href={share.url}
        target="_blank"
        rel="noreferrer"
        aria-label="打开分享链接"
        title="打开分享链接"
        className="hover:bg-muted focus-visible:ring-ring inline-flex size-8 items-center justify-center rounded-md focus-visible:ring-2"
      >
        <ExternalLink className="size-4" aria-hidden="true" />
      </a>
      <ShareCopyButton url={share.url} />
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="撤销分享链接"
        title="撤销分享链接"
        onClick={onRevoke}
      >
        <Trash2 aria-hidden="true" />
      </Button>
    </div>
  );
}

function ShareCopyButton({ url }: { url: string }) {
  const [copyState, setCopyState] = useState<"success" | "error" | null>(null);
  useEffect(() => {
    if (copyState === null) return;
    const timer = window.setTimeout(() => setCopyState(null), 2_000);
    return () => window.clearTimeout(timer);
  }, [copyState]);
  const CopyIcon =
    copyState === "success"
      ? Check
      : copyState === "error"
        ? CircleAlert
        : Copy;
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      className="shrink-0"
      aria-label={
        copyState === "success"
          ? "分享链接已复制"
          : copyState === "error"
            ? "复制分享链接失败，请重试"
            : "复制分享链接"
      }
      title={copyState === "error" ? "复制失败，请重试" : "复制分享链接"}
      data-copy-state={copyState ?? undefined}
      onClick={() => {
        setCopyState(null);
        void copyText(url).then((copied) =>
          setCopyState(copied ? "success" : "error"),
        );
      }}
    >
      <CopyIcon
        aria-hidden="true"
        className={
          copyState === "success"
            ? "text-[color:var(--m2h-copy-success)]"
            : copyState === "error"
              ? "text-[color:var(--m2h-copy-danger)]"
              : undefined
        }
      />
    </Button>
  );
}

function ConfirmRevoke({
  share,
  onCancel,
  onRevoked,
  modal = false,
}: {
  share: DocumentShare;
  onCancel(): void;
  onRevoked(): void;
  modal?: boolean;
}) {
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  async function confirm() {
    setPending(true);
    setFailed(false);
    try {
      await revokeShare(share.id);
      onRevoked();
    } catch {
      setFailed(true);
      setPending(false);
    }
  }
  const description = `撤销「${share.title}」的分享后，原链接立即失效。`;
  const controls = (
    <>
      {failed ? (
        <p role="alert" className="text-destructive text-sm">
          无法撤销分享，请重试。
        </p>
      ) : null}
      <div className="flex justify-end gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={pending}
          onClick={onCancel}
        >
          取消
        </Button>
        <Button
          variant="destructive"
          size="sm"
          disabled={pending}
          onClick={() => void confirm()}
        >
          确认撤销
        </Button>
      </div>
    </>
  );
  if (modal) {
    return (
      <Dialog
        open
        onOpenChange={(open) => {
          if (!open && !pending) onCancel();
        }}
      >
        <DialogContent
          forceBackdrop
          showCloseButton={!pending}
          overlayClassName="z-[60] bg-black/30 supports-backdrop-filter:backdrop-blur-sm"
          className="top-1/2 z-[61] -translate-y-1/2 gap-4 p-5 data-ending-style:-translate-y-[calc(50%-0.5rem)] data-starting-style:-translate-y-[calc(50%-0.5rem)] sm:max-w-lg"
        >
          <DialogTitle>撤销分享</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
          {controls}
        </DialogContent>
      </Dialog>
    );
  }
  return (
    <div className="grid gap-2 rounded-md border p-3 text-sm">
      <p>{description}</p>
      {controls}
    </div>
  );
}

const shareDetailRowClass =
  "grid grid-cols-[7rem_minmax(0,1fr)] items-center gap-3 px-3 py-3";
const shareDetailLabelClass = "text-muted-foreground flex items-center gap-2";

const shareDialogContentClass =
  "top-1/2 max-w-xl -translate-y-1/2 gap-5 overflow-y-auto p-5 sm:p-6";

function ShareDocumentDetails({
  target,
  children,
}: {
  target: ShareTarget;
  children: ReactNode;
}) {
  return (
    <dl className="bg-muted/40 divide-border grid auto-rows-fr divide-y rounded-lg border text-left text-sm">
      <div className={shareDetailRowClass}>
        <dt className={shareDetailLabelClass}>
          <FileText aria-hidden="true" className="size-4 shrink-0" />
          文档名称
        </dt>
        <dd className="border-l border-transparent pl-3 font-medium wrap-anywhere">
          {target.title}
        </dd>
      </div>
      <div className={shareDetailRowClass}>
        <dt className={shareDetailLabelClass}>
          <FolderOpen aria-hidden="true" className="size-4 shrink-0" />
          文档路径
        </dt>
        <dd>
          <code className="bg-muted text-muted-foreground inline-block max-w-full rounded border border-transparent px-3 py-0.5 font-mono text-xs wrap-anywhere">
            {target.documentRef.path}
          </code>
        </dd>
      </div>
      {children}
    </dl>
  );
}

function SharedDocumentDialog({
  target,
  share,
  onClose,
  onRevoked,
}: {
  target: ShareTarget;
  share: DocumentShare;
  onClose(): void;
  onRevoked(): void;
}) {
  const [confirming, setConfirming] = useState(false);
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent
        showCloseButton={false}
        className={shareDialogContentClass}
      >
        <DialogTitle>已分享文档</DialogTitle>
        <ShareDocumentDetails target={target}>
          <div className={shareDetailRowClass}>
            <dt>
              <label
                className={shareDetailLabelClass}
                htmlFor="share-public-link"
              >
                <Link2 aria-hidden="true" className="size-4 shrink-0" />
                公开链接
              </label>
            </dt>
            <dd className="flex min-w-0 items-center gap-1">
              <Input
                id="share-public-link"
                readOnly
                className="min-w-0 pr-2 pl-3 font-mono"
                value={share.url}
                onFocus={(event) => event.currentTarget.select()}
              />
              <ShareCopyButton url={share.url} />
            </dd>
          </div>
        </ShareDocumentDetails>
        {confirming ? (
          <ConfirmRevoke
            share={share}
            onCancel={() => setConfirming(false)}
            onRevoked={onRevoked}
          />
        ) : (
          <div className="flex justify-end gap-2">
            <Button variant="destructive" onClick={() => setConfirming(true)}>
              删除
            </Button>
            <a
              className={buttonVariants()}
              href={share.url}
              target="_blank"
              rel="noreferrer"
            >
              打开
            </a>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

export function ShareDialog({
  target,
  onClose,
}: {
  target: ShareTarget;
  onClose(): void;
}) {
  const { shares, setShares, error, now, retry } = useShareList(
    true,
    target.documentRef,
  );
  const [duration, setDuration] = useState("30m");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const active = shares?.find(
    (item) =>
      item.documentRef.rootId === target.documentRef.rootId &&
      item.documentRef.path === target.documentRef.path &&
      (item.expiresAt === 0 || item.expiresAt > now),
  );
  async function create() {
    setCreating(true);
    setCreateError(null);
    try {
      const share = await createShare(target.documentRef, duration);
      setShares((items) => [
        ...(items ?? []).filter((item) => item.id !== share.id),
        share,
      ]);
    } catch (error) {
      setCreateError(
        error instanceof APIError && error.status === 422
          ? "文档的 Frontmatter 无效。修正文档后再创建分享。"
          : "无法创建分享。请确认文档仍可分享，然后重试。",
      );
    } finally {
      setCreating(false);
    }
  }
  if (active) {
    return (
      <SharedDocumentDialog
        key={active.id}
        target={target}
        share={active}
        onClose={onClose}
        onRevoked={() => {
          setShares(
            (items) => items?.filter((item) => item.id !== active.id) ?? [],
          );
        }}
      />
    );
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent
        showCloseButton={false}
        className={shareDialogContentClass}
      >
        <div className="space-y-1">
          <DialogTitle>确认分享文档？</DialogTitle>
          <DialogDescription>
            分享后，此文档将在有效期内可公开访问
          </DialogDescription>
        </div>
        <ShareDocumentDetails target={target}>
          <div className={shareDetailRowClass}>
            <dt>
              <label className={shareDetailLabelClass} htmlFor="share-duration">
                <Clock3 aria-hidden="true" className="size-4 shrink-0" />
                分享有效期
              </label>
            </dt>
            <dd className="relative w-44 max-w-full">
              <select
                id="share-duration"
                value={duration}
                onChange={(event) => setDuration(event.target.value)}
                className="border-border bg-background h-9 w-full appearance-none rounded-md border pr-9 pl-3 text-left"
              >
                {expiryOptions.map(({ id, label }) => (
                  <option key={id} value={id}>
                    {label}
                  </option>
                ))}
              </select>
              <ChevronDown
                aria-hidden="true"
                className="text-muted-foreground pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2"
              />
            </dd>
          </div>
        </ShareDocumentDetails>
        <div className="grid gap-3">
          {shares === null && !error ? (
            <p className="text-muted-foreground text-sm" role="status">
              正在检查已有分享
            </p>
          ) : null}
          {error ? (
            <div
              className="flex items-center justify-between gap-2 text-sm"
              role="alert"
            >
              无法加载分享状态。
              <Button variant="outline" size="sm" onClick={retry}>
                重试
              </Button>
            </div>
          ) : null}
          {createError !== null ? (
            <p role="alert" className="text-destructive text-sm">
              {createError}
            </p>
          ) : null}
          <div className="flex justify-end gap-2">
            <DialogClose render={<Button variant="outline" />}>
              取消
            </DialogClose>
            <Button
              disabled={creating || shares === null || error}
              onClick={() => void create()}
            >
              {creating ? "分享中" : "分享"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

export function ShareSettings({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange(open: boolean): void;
}) {
  const { shares, setShares, error, now, retry } = useShareList(open);
  const [sort, setSort] = useState<{
    key: "title" | "expiry";
    ascending: boolean;
  }>({
    key: "expiry",
    ascending: true,
  });
  const [selected, setSelected] = useState<DocumentShare | null>(null);
  const ordered = useMemo(
    () =>
      (shares ?? []).toSorted(
        (a, b) =>
          (sort.ascending ? 1 : -1) *
          (sort.key === "title"
            ? a.title.localeCompare(b.title, "zh-CN")
            : (a.expiresAt || Number.MAX_SAFE_INTEGER) -
              (b.expiresAt || Number.MAX_SAFE_INTEGER)),
      ),
    [shares, sort],
  );
  const active = ordered.filter(
    (share) => share.expiresAt === 0 || share.expiresAt > now,
  );
  const headers = [
    ["title", "文档名称"],
    ["expiry", "剩余有效期"],
  ] as const;
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setSelected(null);
        onOpenChange(next);
      }}
    >
      <DialogContent
        aria-describedby={undefined}
        className="top-[10dvh] h-[80dvh] max-h-[80dvh] gap-4 p-5 sm:max-w-5xl"
      >
        <DialogTitle className="pr-8">分享管理</DialogTitle>
        {error ? (
          <div role="alert" className="flex items-center gap-3 text-sm">
            无法加载分享列表。
            <Button variant="outline" size="sm" onClick={retry}>
              重试
            </Button>
          </div>
        ) : null}
        {shares === null && !error ? (
          <p role="status" className="text-muted-foreground text-sm">
            正在加载分享
          </p>
        ) : null}
        {shares !== null && active.length === 0 && !error ? (
          <p className="text-muted-foreground py-10 text-center text-sm">
            暂无有效分享
          </p>
        ) : null}
        {selected ? (
          <ConfirmRevoke
            key={selected.id}
            modal
            share={selected}
            onCancel={() => setSelected(null)}
            onRevoked={() => {
              setShares(
                (items) =>
                  items?.filter((item) => item.id !== selected.id) ?? [],
              );
              setSelected(null);
            }}
          />
        ) : null}
        {active.length > 0 ? (
          <div className="min-h-0 flex-1 overflow-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-popover text-muted-foreground sticky top-0">
                <tr>
                  {headers.map(([key, label]) => {
                    const isSorted = sort.key === key;
                    const SortIcon = isSorted
                      ? sort.ascending
                        ? ArrowUp
                        : ArrowDown
                      : ArrowUpDown;
                    return (
                      <th
                        key={key}
                        scope="col"
                        className={
                          key === "expiry"
                            ? "text-center whitespace-nowrap"
                            : undefined
                        }
                        aria-sort={
                          isSorted
                            ? sort.ascending
                              ? "ascending"
                              : "descending"
                            : "none"
                        }
                      >
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() =>
                            setSort((current) => ({
                              key,
                              ascending:
                                key !== current.key || !current.ascending,
                            }))
                          }
                        >
                          {label}
                          <SortIcon aria-hidden="true" className="size-3.5" />
                        </Button>
                      </th>
                    );
                  })}
                  <th scope="col" className="px-2 text-center">
                    操作
                  </th>
                </tr>
              </thead>
              <tbody>
                {active.map((share) => (
                  <tr key={share.id} className="border-t align-middle">
                    <td className="p-2">
                      <p className="font-medium break-all">{share.title}</p>
                      <p className="text-muted-foreground text-xs break-all">
                        {share.documentRef.path}
                      </p>
                    </td>
                    <td className="p-2 text-center whitespace-nowrap">
                      <ShareExpiry share={share} />
                    </td>
                    <td className="p-2 text-center">
                      <ShareActions
                        share={share}
                        onRevoke={() => setSelected(share)}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
