import { render, screen } from "@testing-library/react";
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
    loginPending: false,
    loginError: null,
    login: vi.fn(),
    logout: vi.fn(),
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
});
