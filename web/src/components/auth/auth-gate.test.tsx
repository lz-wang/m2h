import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { UseAuthResult } from "../../hooks/use-auth";
import { AuthGate } from "./auth-gate";

vi.mock("../../App", () => ({
  App: () => {
    // Record that the reader mounted at all — the gate's whole contract
    // is which phases mount it.
    readerMounts.push(1);
    return <div data-testid="reader-app" />;
  },
}));

const readerMounts: number[] = [];

function authState(overrides: Partial<UseAuthResult>): UseAuthResult {
  return {
    phase: "off",
    authMode: "none",
    features: { annotations: false, tokens: false },
    identity: null,
    bootError: null,
    logoutError: null,
    loginPending: false,
    loginError: null,
    login: vi.fn(),
    logout: vi.fn(),
    retryBoot: vi.fn(),
    ...overrides,
  };
}

describe("AuthGate", () => {
  it("renders nothing while the auth state is unknown", () => {
    const { container } = render(
      <AuthGate useAuthHook={() => authState({ phase: "booting" })} />,
    );
    expect(container.textContent).toBe("");
    expect(readerMounts).toHaveLength(0);
  });

  it("mounts the reader once the phase settles", () => {
    render(<AuthGate useAuthHook={() => authState({ phase: "off" })} />);
    expect(screen.getByTestId("reader-app")).toBeTruthy();
  });

  it("shows the login page for anonymous sessions", () => {
    render(
      <AuthGate
        useAuthHook={() => authState({ phase: "anonymous", authMode: "local" })}
      />,
    );
    expect(screen.getByRole("heading", { name: "登录 m2h" })).toBeTruthy();
    expect(screen.queryByTestId("reader-app")).toBeNull();
  });

  it("shows a retry screen when the boot probe failed — never the reader", () => {
    const retryBoot = vi.fn();
    render(
      <AuthGate
        useAuthHook={() =>
          authState({
            phase: "error",
            authMode: "local",
            bootError: "无法连接服务，请检查网络后重试。",
            retryBoot,
          })
        }
      />,
    );
    expect(screen.getByRole("alert").textContent).toContain("无法连接服务");
    expect(screen.queryByTestId("reader-app")).toBeNull();
    const retry = screen.getByRole("button", { name: /重试/ });
    expect(retry).toBeTruthy();
  });

  it("fires the boot retry from the error screen", async () => {
    const retryBoot = vi.fn().mockResolvedValue(undefined);
    render(
      <AuthGate
        useAuthHook={() =>
          authState({ phase: "error", authMode: "local", retryBoot })
        }
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /重试/ }));
    expect(retryBoot).toHaveBeenCalledTimes(1);
  });

  it("surfaces a failed logout over the reader and offers a retry", async () => {
    const logout = vi.fn();
    render(
      <AuthGate
        useAuthHook={() =>
          authState({
            phase: "authenticated",
            authMode: "local",
            identity: {
              username: "alice",
              displayName: "Alice",
              provider: "local",
            },
            logoutError: "注销失败，会话可能仍然有效，请重试。",
            logout,
          })
        }
      />,
    );
    // The reader stays — the session was never confirmed gone.
    expect(screen.getByTestId("reader-app")).toBeTruthy();
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain("注销失败");
    await userEvent.click(screen.getByRole("button", { name: "重试注销" }));
    expect(logout).toHaveBeenCalledTimes(1);
  });
});
