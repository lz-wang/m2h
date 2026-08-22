import { afterEach, describe, expect, it, vi } from "vitest";

import { copyText } from "./clipboard";

const restorees: Array<() => void> = [];

function replaceProperty(
  target: object,
  property: PropertyKey,
  value: unknown,
): void {
  const descriptor = Object.getOwnPropertyDescriptor(target, property);
  Object.defineProperty(target, property, { configurable: true, value });
  restorees.push(() => {
    if (descriptor === undefined) {
      Reflect.deleteProperty(target, property);
      return;
    }
    Object.defineProperty(target, property, descriptor);
  });
}

afterEach(() => {
  for (const restore of restorees.splice(0)) {
    restore();
  }
});

describe("copyText", () => {
  it("writes through the Clipboard API in a secure context", async () => {
    replaceProperty(window, "isSecureContext", true);
    const writeText = vi
      .fn<(value: string) => Promise<void>>()
      .mockResolvedValue(undefined);
    replaceProperty(navigator, "clipboard", { writeText });
    const execCommand = vi.fn(() => true);
    replaceProperty(document, "execCommand", execCommand);

    await expect(copyText("# 计划 doc.md")).resolves.toBe(true);
    expect(writeText).toHaveBeenCalledWith("# 计划 doc.md");
    expect(execCommand).not.toHaveBeenCalled();
  });

  it("falls back to execCommand when the Clipboard API rejects", async () => {
    replaceProperty(window, "isSecureContext", true);
    const writeText = vi
      .fn<(value: string) => Promise<void>>()
      .mockRejectedValue(new DOMException("denied", "NotAllowedError"));
    replaceProperty(navigator, "clipboard", { writeText });
    const execCommand = vi.fn(() => true);
    replaceProperty(document, "execCommand", execCommand);

    await expect(copyText("body")).resolves.toBe(true);
    expect(execCommand).toHaveBeenCalledWith("copy");
  });

  it("uses execCommand directly on plain HTTP previews", async () => {
    replaceProperty(window, "isSecureContext", false);
    const writeText = vi.fn<(value: string) => Promise<void>>();
    replaceProperty(navigator, "clipboard", { writeText });
    const execCommand = vi.fn(() => true);
    replaceProperty(document, "execCommand", execCommand);

    await expect(copyText("http://192.168.1.4:8793 reader")).resolves.toBe(
      true,
    );
    expect(writeText).not.toHaveBeenCalled();
    expect(execCommand).toHaveBeenCalledWith("copy");
  });

  it("reports failure when every path fails and cleans up the textarea", async () => {
    replaceProperty(window, "isSecureContext", false);
    const execCommand = vi.fn(() => {
      throw new Error("unsupported");
    });
    replaceProperty(document, "execCommand", execCommand);

    await expect(copyText("lost")).resolves.toBe(false);
    expect(document.querySelectorAll("textarea")).toHaveLength(0);
  });
});
