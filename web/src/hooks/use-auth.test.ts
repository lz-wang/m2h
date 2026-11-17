import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { APIError, unauthorizedEventName } from "../api";
import { useAuth } from "./use-auth";

const fetchMock = vi.fn<typeof fetch>();

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockReset();
});

describe("useAuth", () => {
  it("keeps the default mode off — no session probe, no login page", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        authMode: "none",
        features: { annotations: false, tokens: false },
      }),
    );
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("off"));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(result.current.identity).toBeNull();
  });

  it("boots authenticated when the session is valid", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          authMode: "local",
          features: { annotations: false, tokens: false },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          username: "alice",
          displayName: "Alice",
          provider: "local",
        }),
      );
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("authenticated"));
    expect(result.current.identity?.username).toBe("alice");
    expect(result.current.authMode).toBe("local");
  });

  it("boots anonymous when the session check answers 401", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          authMode: "ldap",
          features: { annotations: false, tokens: false },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ error: "authentication required" }, 401),
      );
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("anonymous"));
  });

  it("falls back to the reader when the config cannot be loaded", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("network down"));
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("off"));
  });

  it("login stores the identity on success", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          authMode: "local",
          features: { annotations: false, tokens: false },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ error: "authentication required" }, 401),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          username: "alice",
          displayName: "Alice",
          provider: "local",
        }),
      );
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("anonymous"));

    let success = false;
    await act(async () => {
      success = await result.current.login("alice", "password-1");
    });
    expect(success).toBe(true);
    expect(result.current.phase).toBe("authenticated");
    expect(result.current.loginError).toBeNull();

    const loginCall = fetchMock.mock.calls[2];
    expect(loginCall?.[0]).toBe("/api/auth/login");
    expect(String(loginCall?.[1]?.method)).toBe("POST");
    const body = JSON.parse(String(loginCall?.[1]?.body)) as {
      username: string;
      password: string;
    };
    expect(body).toEqual({ username: "alice", password: "password-1" });
  });

  it("login maps 401 and 429 to user-facing messages", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          authMode: "local",
          features: { annotations: false, tokens: false },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ error: "authentication required" }, 401),
      )
      .mockResolvedValueOnce(
        jsonResponse({ error: "invalid credentials" }, 401),
      )
      .mockResolvedValueOnce(
        jsonResponse({ error: "too many failed attempts" }, 429),
      );
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("anonymous"));

    await act(async () => {
      await result.current.login("alice", "wrong");
    });
    expect(result.current.phase).toBe("anonymous");
    expect(result.current.loginError).toBe("用户名或密码错误。");

    await act(async () => {
      await result.current.login("alice", "wrong");
    });
    expect(result.current.loginError).toBe("尝试次数过多，请稍后再试。");
  });

  it("logout returns to the login page", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          authMode: "local",
          features: { annotations: false, tokens: false },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          username: "alice",
          displayName: "Alice",
          provider: "local",
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ ok: true }));
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("authenticated"));

    await act(async () => {
      await result.current.logout();
    });
    expect(result.current.phase).toBe("anonymous");
    expect(result.current.identity).toBeNull();
  });

  it("a mid-reading 401 event flips an authenticated session to anonymous", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          authMode: "local",
          features: { annotations: false, tokens: false },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          username: "alice",
          displayName: "Alice",
          provider: "local",
        }),
      );
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("authenticated"));

    act(() => {
      window.dispatchEvent(new CustomEvent(unauthorizedEventName));
    });
    expect(result.current.phase).toBe("anonymous");
    expect(result.current.identity).toBeNull();
  });

  it("never treats a config 401 as a session loss", async () => {
    // requestJSON dispatches the unauthorized event for non-auth routes
    // only; the boot probe hits /api/config and /api/auth/me, neither of
    // which may flip a not-yet-authenticated gate.
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          authMode: "local",
          features: { annotations: false, tokens: false },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ error: "authentication required" }, 401),
      );
    const { result } = renderHook(() => useAuth());
    await waitFor(() => expect(result.current.phase).toBe("anonymous"));
  });

  it("exposes APIError status for callers that need it", () => {
    const error = new APIError(401, "invalid credentials");
    expect(error.status).toBe(401);
    expect(error.message).toBe("invalid credentials");
  });
});
