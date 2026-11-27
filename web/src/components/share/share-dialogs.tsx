import { Copy, ExternalLink, Link2, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";

import {
  createShare,
  type DocumentRef,
  type DocumentShare,
  listShares,
  revokeShare,
} from "../../api";
import { copyText } from "../../lib/clipboard";
import { expiryOptions } from "../../lib/expiry";
import { Button } from "../ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "../ui/dialog";
import { Input } from "../ui/input";

export interface ShareTarget {
  documentRef: DocumentRef;
  title: string;
}

function useShareList(open: boolean) {
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
    const timer = window.setInterval(() => setNow(Date.now()), 1_000);
    void listShares(controller.signal).then(
      (items) => {
        if (!controller.signal.aborted) setShares(items);
      },
      () => {
        if (!controller.signal.aborted) setError(true);
      },
    );
    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [open, revision]);
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

function ShareExpiry({ share, now }: { share: DocumentShare; now: number }) {
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
  const [copyStatus, setCopyStatus] = useState("");
  return (
    <div className="flex items-center gap-1">
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
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="复制分享链接"
        title="复制分享链接"
        onClick={() =>
          void copyText(share.url).then((copied) =>
            setCopyStatus(copied ? "已复制" : "复制失败"),
          )
        }
      >
        <Copy aria-hidden="true" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="撤销分享链接"
        title="撤销分享链接"
        onClick={onRevoke}
      >
        <Trash2 aria-hidden="true" />
      </Button>
      <span className="text-muted-foreground text-xs" role="status">
        {copyStatus}
      </span>
    </div>
  );
}

function ConfirmRevoke({
  share,
  onCancel,
  onRevoked,
}: {
  share: DocumentShare;
  onCancel(): void;
  onRevoked(): void;
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
  return (
    <div className="grid gap-2 rounded-md border p-3 text-sm">
      <p>撤销「{share.title}」的分享后，原链接立即失效。</p>
      {failed ? (
        <p role="alert" className="text-destructive">
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
    </div>
  );
}

export function ShareDialog({
  target,
  onClose,
}: {
  target: ShareTarget;
  onClose(): void;
}) {
  const { shares, setShares, error, now, retry } = useShareList(true);
  const [duration, setDuration] = useState("30m");
  const [creating, setCreating] = useState(false);
  const [createFailed, setCreateFailed] = useState(false);
  const [confirmID, setConfirmID] = useState("");
  const active = shares?.find(
    (item) =>
      item.documentRef.rootId === target.documentRef.rootId &&
      item.documentRef.path === target.documentRef.path &&
      (item.expiresAt === 0 || item.expiresAt > now),
  );
  async function create() {
    setCreating(true);
    setCreateFailed(false);
    try {
      const share = await createShare(target.documentRef, duration);
      setShares((items) => [
        ...(items ?? []).filter((item) => item.id !== share.id),
        share,
      ]);
    } catch {
      setCreateFailed(true);
    } finally {
      setCreating(false);
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="top-1/2 max-w-md -translate-y-1/2 gap-4 overflow-y-auto p-5">
        <div className="space-y-1 pr-8">
          <DialogTitle>分享文档</DialogTitle>
          <DialogDescription>{target.title}</DialogDescription>
        </div>
        <div className="bg-muted/60 flex items-start gap-3 rounded-md p-3 text-sm">
          <Link2 className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          <div className="space-y-1">
            <p className="font-medium">任何获得链接的人</p>
            <p className="text-muted-foreground">
              无需登录即可阅读。链接有效期间，正文和附件的后续变更也会公开。
            </p>
          </div>
        </div>
        {active ? (
          <div className="grid gap-3">
            <Input
              readOnly
              aria-label="分享链接"
              value={active.url}
              onFocus={(event) => event.currentTarget.select()}
            />
            <div className="flex items-center justify-between gap-2 text-sm">
              <span className="text-muted-foreground">
                剩余有效期 <ShareExpiry share={active} now={now} />
              </span>
              <ShareActions
                key={active.id}
                share={active}
                onRevoke={() => setConfirmID(active.id)}
              />
            </div>
            {confirmID === active.id ? (
              <ConfirmRevoke
                share={active}
                onCancel={() => setConfirmID("")}
                onRevoked={() => {
                  setShares(
                    (items) =>
                      items?.filter((item) => item.id !== active.id) ?? [],
                  );
                  setConfirmID("");
                }}
              />
            ) : null}
          </div>
        ) : (
          <div className="grid gap-3">
            <label
              className="flex flex-col gap-1.5 text-sm"
              htmlFor="share-duration"
            >
              有效期
              <select
                id="share-duration"
                value={duration}
                onChange={(event) => setDuration(event.target.value)}
                className="border-border bg-background h-9 rounded-md border px-2"
              >
                {expiryOptions.map(({ id, label }) => (
                  <option key={id} value={id}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            {duration === "permanent" ? (
              <p className="text-muted-foreground text-sm">
                永久链接会持续公开这篇文档及其后续修改，直到撤销或失效。
              </p>
            ) : null}
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
            {createFailed ? (
              <p role="alert" className="text-destructive text-sm">
                无法创建分享。请确认文档仍可分享，然后重试。
              </p>
            ) : null}
            <Button
              className="justify-self-end"
              disabled={creating || shares === null || error}
              onClick={() => void create()}
            >
              {creating ? "正在创建" : "创建分享链接"}
            </Button>
          </div>
        )}
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
  const [by, setBy] = useState<"title" | "expiry">("expiry");
  const [direction, setDirection] = useState<1 | -1>(1);
  const [confirmID, setConfirmID] = useState("");
  const active = (shares ?? []).filter(
    (share) => share.expiresAt === 0 || share.expiresAt > now,
  );
  active.sort(
    (a, b) =>
      direction *
      (by === "title"
        ? a.title.localeCompare(b.title)
        : (a.expiresAt || Number.MAX_SAFE_INTEGER) -
          (b.expiresAt || Number.MAX_SAFE_INTEGER)),
  );
  const selected = active.find((item) => item.id === confirmID);
  function sort(next: typeof by) {
    setDirection(by === next && direction === 1 ? -1 : 1);
    setBy(next);
  }
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setConfirmID("");
        onOpenChange(next);
      }}
    >
      <DialogContent className="top-[10dvh] h-[80dvh] max-h-[80dvh] gap-4 p-5 sm:max-w-5xl">
        <div className="space-y-1 pr-8">
          <DialogTitle>分享管理</DialogTitle>
          <DialogDescription>查看或撤销你创建的有效分享。</DialogDescription>
        </div>
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
          <p className="text-muted-foreground text-sm">暂无有效分享。</p>
        ) : null}
        {selected ? (
          <ConfirmRevoke
            key={selected.id}
            share={selected}
            onCancel={() => setConfirmID("")}
            onRevoked={() => {
              setShares(
                (items) =>
                  items?.filter((item) => item.id !== selected.id) ?? [],
              );
              setConfirmID("");
            }}
          />
        ) : null}
        {active.length > 0 ? (
          <div className="min-h-0 flex-1 overflow-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-popover text-muted-foreground sticky top-0">
                <tr>
                  <th
                    scope="col"
                    aria-sort={
                      by === "title"
                        ? direction === 1
                          ? "ascending"
                          : "descending"
                        : "none"
                    }
                  >
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => sort("title")}
                    >
                      文档名称
                    </Button>
                  </th>
                  <th
                    scope="col"
                    className="whitespace-nowrap"
                    aria-sort={
                      by === "expiry"
                        ? direction === 1
                          ? "ascending"
                          : "descending"
                        : "none"
                    }
                  >
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => sort("expiry")}
                    >
                      剩余有效期
                    </Button>
                  </th>
                  <th scope="col" className="px-2">
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
                    <td className="p-2 whitespace-nowrap">
                      <ShareExpiry share={share} now={now} />
                    </td>
                    <td className="p-2">
                      <ShareActions
                        share={share}
                        onRevoke={() => setConfirmID(share.id)}
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
