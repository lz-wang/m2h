import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import * as clipboard from "../../lib/clipboard";
import { ShareDialog, ShareSettings } from "./share-dialogs";

const request = vi.fn<typeof fetch>();
const target = {
  documentRef: { rootId: "root", path: "guide.md" },
  title: "Guide",
};
const id = "a".repeat(43);
const shared = {
  ...target,
  id,
  url: `/share/${id}`,
  createdAt: 1,
  expiresAt: 0,
};

function json(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), { status });
}

beforeEach(() => {
  request.mockReset();
  vi.stubGlobal("fetch", request);
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

it("creates and copies the current document's link", async () => {
  request
    .mockResolvedValueOnce(json([]))
    .mockResolvedValueOnce(json(shared, 201));
  vi.spyOn(clipboard, "copyText").mockResolvedValue(true);
  const user = userEvent.setup();
  render(<ShareDialog target={target} onClose={vi.fn()} />);
  const create = screen.getByRole<HTMLButtonElement>("button", {
    name: "分享",
  });
  expect(
    screen.getByRole<HTMLSelectElement>("combobox", { name: "分享有效期" })
      .value,
  ).toBe("30m");
  await user.selectOptions(screen.getByLabelText("分享有效期"), "permanent");
  expect(screen.getByText("分享后，此文档将在有效期内可公开访问")).toBeTruthy();
  await user.click(create);
  const address = `${location.origin}/share/${id}`;
  const input = await screen.findByRole<HTMLInputElement>("textbox", {
    name: "公开链接",
  });
  expect(input.value).toBe(address);
  const sharedDialog = screen.getByRole("dialog", { name: "已分享文档" });
  expect(
    within(sharedDialog).getByText(target.title, { exact: true }),
  ).toBeTruthy();
  expect(
    within(sharedDialog).getByText(target.documentRef.path, { exact: true }),
  ).toBeTruthy();
  expect(
    within(sharedDialog)
      .getByRole("button", { name: "删除" })
      .querySelector("svg"),
  ).toBeNull();
  expect(
    within(sharedDialog)
      .getByRole("link", { name: "打开" })
      .querySelector("svg"),
  ).toBeNull();
  expect(
    within(sharedDialog).queryByText("分享后，此文档将在有效期内可公开访问"),
  ).toBeNull();
  expect(within(sharedDialog).queryByText(/剩余有效期/)).toBeNull();
  expect(
    within(sharedDialog)
      .getByRole("link", { name: "打开" })
      .getAttribute("href"),
  ).toBe(address);
  expect(
    within(sharedDialog).getByRole("button", { name: "删除" }),
  ).toBeTruthy();
  expect(request.mock.calls[0]?.[0]).toBe(
    "/api/shares?rootId=root&path=guide.md",
  );
  expect(request.mock.calls[1]?.[0]).toBe("/api/shares");
  expect(request.mock.calls[1]?.[1]?.body).toBe(
    JSON.stringify({ documentRef: target.documentRef, duration: "permanent" }),
  );
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  const copy = screen.getByRole("button", { name: "复制分享链接" });
  await act(async () => {
    fireEvent.click(copy);
  });
  expect(clipboard.copyText).toHaveBeenCalledWith(address);
  expect(copy.getAttribute("data-copy-state")).toBe("success");
  expect(copy.querySelector(".lucide-check")).not.toBeNull();
  expect(screen.queryByText("已复制")).toBeNull();
  act(() => vi.advanceTimersByTime(2_000));
  expect(copy.getAttribute("data-copy-state")).toBeNull();
  expect(copy.getAttribute("aria-label")).toBe("复制分享链接");
});

it("cancels the confirmation without creating a share", async () => {
  request.mockResolvedValueOnce(json([]));
  const onClose = vi.fn();
  render(<ShareDialog target={target} onClose={onClose} />);
  const dialog = screen.getByRole("dialog", { name: "确认分享文档？" });
  expect(within(dialog).getByText(target.documentRef.path)).toBeTruthy();
  expect(within(dialog).queryByRole("button", { name: "关闭" })).toBeNull();
  await userEvent.click(within(dialog).getByRole("button", { name: "取消" }));
  expect(onClose).toHaveBeenCalledOnce();
  expect(request).toHaveBeenCalledTimes(1);
});

it("reuses the existing link and allows cancellation of revocation", async () => {
  request.mockResolvedValueOnce(json([shared]));
  const user = userEvent.setup();
  render(<ShareDialog target={target} onClose={vi.fn()} />);
  await screen.findByRole("textbox", { name: "公开链接" });
  expect(screen.queryByRole("button", { name: "分享" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "删除" }));
  await user.click(screen.getByRole("button", { name: "取消" }));
  expect(screen.queryByRole("button", { name: "确认撤销" })).toBeNull();
  expect(request).toHaveBeenCalledTimes(1);
});

it("sorts only active records and removes a revoked share", async () => {
  const temporary = {
    ...shared,
    id: "b".repeat(43),
    url: `/share/${"b".repeat(43)}`,
    title: "Alpha",
    expiresAt: Date.now() + 3_600_000,
  };
  const expired = {
    ...shared,
    id: "c".repeat(43),
    url: `/share/${"c".repeat(43)}`,
    title: "Expired",
    expiresAt: 1,
  };
  request
    .mockResolvedValueOnce(json([shared, temporary, expired]))
    .mockResolvedValueOnce(new Response(null, { status: 204 }));
  vi.spyOn(clipboard, "copyText").mockResolvedValue(true);
  const user = userEvent.setup();
  render(<ShareSettings open onOpenChange={vi.fn()} />);
  const alpha = await screen.findByRole("row", { name: /Alpha/ });
  expect(screen.queryByText("Expired")).toBeNull();
  const name = screen.getByRole("columnheader", { name: "文档名称" });
  const expiry = screen.getByRole("columnheader", { name: "剩余有效期" });
  expect(expiry.getAttribute("aria-sort")).toBe("ascending");
  expect(screen.getAllByRole("row")[1]).toBe(alpha);
  await user.click(within(expiry).getByRole("button"));
  expect(expiry.getAttribute("aria-sort")).toBe("descending");
  expect(screen.getAllByRole("row")[1]?.textContent).toContain("Guide");
  await user.click(within(name).getByRole("button"));
  expect(name.getAttribute("aria-sort")).toBe("ascending");
  expect(screen.getAllByRole("row")[1]).toBe(alpha);
  await user.click(within(name).getByRole("button"));
  expect(name.getAttribute("aria-sort")).toBe("descending");
  expect(screen.getAllByRole("row")[1]?.textContent).toContain("Guide");

  const copy = within(alpha).getByRole("button", { name: "复制分享链接" });
  await user.click(copy);
  expect(clipboard.copyText).toHaveBeenCalledWith(
    new URL(temporary.url, location.origin).href,
  );
  expect(copy.querySelector(".lucide-check")).not.toBeNull();
  expect(screen.queryByText("已复制")).toBeNull();

  const revoke = within(alpha).getByRole("button", { name: "撤销分享链接" });
  await user.click(revoke);
  const confirmation = screen.getByRole("dialog", { name: "撤销分享" });
  expect(within(confirmation).getByText(/撤销「Alpha」/)).toBeTruthy();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog", { name: "撤销分享" })).toBeNull();
  expect(request).toHaveBeenCalledTimes(1);

  await user.click(revoke);
  await user.click(
    within(screen.getByRole("dialog", { name: "撤销分享" })).getByRole(
      "button",
      { name: "确认撤销" },
    ),
  );
  expect(await screen.findByText("Guide")).toBeTruthy();
  await waitFor(() => expect(screen.queryByText("Alpha")).toBeNull());
  expect(request.mock.calls[1]?.[0]).toBe(`/api/shares/${temporary.id}`);
  expect(request.mock.calls[1]?.[1]?.method).toBe("DELETE");
});

it("retries failed list and create requests", async () => {
  request
    .mockRejectedValueOnce(new Error("offline"))
    .mockResolvedValueOnce(json([]))
    .mockResolvedValueOnce(json({}, 503));
  const user = userEvent.setup();
  render(<ShareDialog target={target} onClose={vi.fn()} />);
  await user.click(await screen.findByRole("button", { name: "重试" }));
  const create = screen.getByRole<HTMLButtonElement>("button", {
    name: "分享",
  });
  await waitFor(() => expect(create.disabled).toBe(false));
  await user.click(create);
  expect(await screen.findByText(/无法创建分享/)).toBeTruthy();
});

it("refreshes expiry labels at their display boundary and removes expired rows", async () => {
  vi.useFakeTimers({ toFake: ["Date", "setTimeout", "clearTimeout"] });
  const expiresAt = Date.now() + 120_100;
  const temporary = {
    ...shared,
    title: "Temporary",
    id: "b".repeat(43),
    url: `/share/${"b".repeat(43)}`,
    expiresAt,
  };
  request.mockResolvedValueOnce(json([shared, temporary]));
  await act(async () => {
    render(<ShareSettings open onOpenChange={vi.fn()} />);
  });
  const row = screen.getByRole("row", { name: /Temporary/ });
  expect(within(row).getByText("2m", { exact: true })).toBeTruthy();
  act(() => vi.advanceTimersByTime(101));
  expect(within(row).getByText("1m", { exact: true })).toBeTruthy();
  act(() => vi.advanceTimersByTime(119_998));
  expect(within(row).getByText("不足 1 分钟", { exact: true })).toBeTruthy();
  act(() => vi.advanceTimersByTime(1));
  expect(screen.queryByRole("row", { name: /Temporary/ })).toBeNull();
  expect(screen.getByText("永久", { exact: true })).toBeTruthy();
});
