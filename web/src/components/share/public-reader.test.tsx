import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import { PublicReader } from "./public-reader";

const id = "a".repeat(43);

afterEach(() => {
  vi.unstubAllGlobals();
});

it("loads only the shared document and provides reader controls", async () => {
  const request = vi.fn<typeof fetch>().mockResolvedValueOnce(
    new Response(
      JSON.stringify({
        title: "Guide",
        html: '<h1 id="guide">Guide</h1><p>Public text.</p>',
        toc: [{ id: "guide", text: "Guide", level: 1 }],
      }),
    ),
  );
  vi.stubGlobal("fetch", request);
  render(<PublicReader id={id} />);
  const main = screen.getByRole("main");
  expect(await within(main).findByText("Public text.")).toBeTruthy();
  expect(
    await within(main).findByRole("navigation", { name: "文档目录" }),
  ).toBeTruthy();
  expect(request.mock.calls.map(([path]) => path)).toEqual([
    `/api/public/shares/${id}`,
  ]);
  expect(screen.queryByRole("button", { name: "设置" })).toBeNull();
});

it("retries a service failure and displays the resulting unavailable link", async () => {
  const request = vi
    .fn<typeof fetch>()
    .mockResolvedValueOnce(new Response(null, { status: 500 }))
    .mockResolvedValueOnce(new Response(null, { status: 404 }));
  vi.stubGlobal("fetch", request);
  render(<PublicReader id={id} />);
  await userEvent.click(
    await screen.findByRole("button", { name: "重新加载" }),
  );
  expect(await screen.findByText("此分享链接不可用。")).toBeTruthy();
  expect(request).toHaveBeenCalledTimes(2);
});
