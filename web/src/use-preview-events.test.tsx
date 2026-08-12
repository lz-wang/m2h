import { render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { usePreviewEvents } from "./use-preview-events";

function HookHarness({ handler }: { handler: () => void }) {
  usePreviewEvents(handler);
  return null;
}

describe("usePreviewEvents", () => {
  afterEach(() => {
    MockEventSource.reset();
  });

  it("invokes the callback only for document-changed events", () => {
    const handler = vi.fn();
    vi.stubGlobal("EventSource", MockEventSource);
    render(<HookHarness handler={handler} />);

    expect(MockEventSource.last).not.toBeNull();
    expect(MockEventSource.last?.url).toBe("/api/events");
    expect(handler).not.toHaveBeenCalled();

    MockEventSource.last?.dispatch("document-changed");
    expect(handler).toHaveBeenCalledTimes(1);

    // Unrelated SSE events must not trigger a reload.
    MockEventSource.last?.dispatch("keep-alive");
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it("closes the stream on unmount", () => {
    const handler = vi.fn();
    vi.stubGlobal("EventSource", MockEventSource);
    const { unmount } = render(<HookHarness handler={handler} />);
    const source = MockEventSource.last;
    unmount();
    expect(source?.closed).toBe(true);
  });

  it("does not reconnect when the callback identity is stable", () => {
    const handler = vi.fn();
    vi.stubGlobal("EventSource", MockEventSource);
    const { rerender } = render(<HookHarness handler={handler} />);
    const first = MockEventSource.last;
    rerender(<HookHarness handler={handler} />);
    expect(MockEventSource.last).toBe(first);
  });
});

class MockEventSource {
  static last: MockEventSource | null = null;

  static reset(): void {
    MockEventSource.last = null;
  }

  readonly url: string;
  closed = false;
  private readonly listeners = new Map<string, Set<(event: Event) => void>>();

  constructor(url: string) {
    this.url = url;
    MockEventSource.last = this;
  }

  addEventListener(type: string, listener: (event: Event) => void): void {
    let listeners = this.listeners.get(type);
    if (listeners === undefined) {
      listeners = new Set();
      this.listeners.set(type, listeners);
    }
    listeners.add(listener);
  }

  removeEventListener(type: string, listener: (event: Event) => void): void {
    this.listeners.get(type)?.delete(listener);
  }

  close(): void {
    this.closed = true;
  }

  dispatch(type: string): void {
    const event = new Event(type);
    this.listeners.get(type)?.forEach((listener) => {
      listener(event);
    });
  }
}
