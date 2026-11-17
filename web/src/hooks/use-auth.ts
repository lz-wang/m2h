import { useCallback, useEffect, useRef, useState } from "react";

import {
  APIError,
  type AuthIdentity,
  type AuthMode,
  fetchCurrentIdentity,
  fetchSiteConfig,
  loginRequest,
  logoutRequest,
  type SiteConfig,
  unauthorizedEventName,
} from "../api";

// AuthPhase is the gate's lifecycle: the configuration and session are
// probed once at boot; "off" is the default unauthenticated mode (no login
// page ever shows), "anonymous" waits on the login form, "authenticated"
// unlocks the reader, and "error" parks the gate when the capability
// probe itself failed — an unknown auth mode must never be mistaken for
// "no auth required".
export type AuthPhase =
  | "booting"
  | "off"
  | "anonymous"
  | "authenticated"
  | "error";

export interface UseAuthResult {
  phase: AuthPhase;
  authMode: AuthMode;
  // The deployment's capability flags from /api/config (annotations,
  // tokens); both false in the default mode.
  features: SiteConfig["features"];
  identity: AuthIdentity | null;
  // Non-null while the boot probe failed and the gate shows its retry
  // screen; cleared by a successful retryBoot.
  bootError: string | null;
  // Non-null while a logout request failed on the wire: the session is
  // still live server-side, so the reader stays and the user may retry.
  logoutError: string | null;
  // Non-null while a login request is in flight; the login form disables
  // its submit button to prevent double submits.
  loginPending: boolean;
  // The last login failure as a user-facing message; cleared when the form
  // or a retry changes.
  loginError: string | null;
  login: (username: string, password: string) => Promise<boolean>;
  logout: () => Promise<void>;
  // Re-runs the boot probe after a failure.
  retryBoot: () => Promise<void>;
}

// useAuth drives the whole authentication state machine of the WebUI: the
// boot probe (config → session), login, logout, and the mid-reading
// session-loss flip. It touches no document API — the reader mounts only
// after the phase is settled, so no protected request leaves before the
// login state is known.
export function useAuth(): UseAuthResult {
  const [phase, setPhase] = useState<AuthPhase>("booting");
  const [authMode, setAuthMode] = useState<AuthMode>("none");
  const [features, setFeatures] = useState<SiteConfig["features"]>({
    annotations: false,
    tokens: false,
  });
  const [identity, setIdentity] = useState<AuthIdentity | null>(null);
  const [bootError, setBootError] = useState<string | null>(null);
  const [logoutError, setLogoutError] = useState<string | null>(null);
  const [loginPending, setLoginPending] = useState(false);
  const [loginError, setLoginError] = useState<string | null>(null);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  // probeBoot resolves the deployment's auth mode and the caller's
  // session. It is the boot path and the retry path in one: a network
  // failure answering the capability probe is an ERROR, never a silent
  // downgrade to the unauthenticated reader — a server that actually runs
  // authenticated would then answer every document request with 401s.
  const probeBoot = useCallback(async (signal?: AbortSignal) => {
    setPhase("booting");
    let config: SiteConfig;
    try {
      config = await fetchSiteConfig(signal);
    } catch (error) {
      if (signal?.aborted || !mounted.current) {
        return;
      }
      setBootError(
        error instanceof Error && error.message !== ""
          ? error.message
          : "无法连接服务，请检查网络后重试。",
      );
      setPhase("error");
      return;
    }
    if (signal?.aborted || !mounted.current) {
      return;
    }
    setBootError(null);
    setAuthMode(config.authMode);
    setFeatures(config.features);
    if (config.authMode === "none") {
      setPhase("off");
      return;
    }
    try {
      const current = await fetchCurrentIdentity(signal);
      if (signal?.aborted || !mounted.current) {
        return;
      }
      if (current !== null) {
        setIdentity(current);
        setPhase("authenticated");
      } else {
        setPhase("anonymous");
      }
    } catch {
      if (!signal?.aborted && mounted.current) {
        // The session check failing hard is an anonymous boot; the next
        // login attempt surfaces real errors.
        setPhase("anonymous");
      }
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void probeBoot(controller.signal);
    return () => {
      controller.abort();
    };
  }, [probeBoot]);

  const retryBoot = useCallback(async () => {
    await probeBoot();
  }, [probeBoot]);

  // A protected request answering 401 mid-reading ends the session: swap
  // straight to the login page instead of leaving the reader accumulating
  // auth errors.
  useEffect(() => {
    const handleUnauthorized = () => {
      if (mounted.current) {
        setIdentity(null);
        setPhase((current) =>
          current === "authenticated" ? "anonymous" : current,
        );
      }
    };
    window.addEventListener(unauthorizedEventName, handleUnauthorized);
    return () =>
      window.removeEventListener(unauthorizedEventName, handleUnauthorized);
  }, []);

  const login = useCallback(
    async (username: string, password: string): Promise<boolean> => {
      setLoginPending(true);
      setLoginError(null);
      try {
        const next = await loginRequest(username, password);
        if (mounted.current) {
          setIdentity(next);
          setPhase("authenticated");
        }
        return true;
      } catch (error) {
        if (mounted.current) {
          setLoginError(loginErrorMessage(error));
        }
        return false;
      } finally {
        if (mounted.current) {
          setLoginPending(false);
        }
      }
    },
    [],
  );

  const logout = useCallback(async (): Promise<void> => {
    try {
      await logoutRequest();
    } catch (error) {
      if (error instanceof APIError && error.status === 401) {
        // The server no longer knows the session — it is as logged out as
        // it ever will be.
        setLogoutError(null);
        if (mounted.current) {
          setIdentity(null);
          setPhase("anonymous");
        }
        return;
      }
      // Without server confirmation the session cookie may still be live:
      // say so instead of silently pretending the logout succeeded.
      if (mounted.current) {
        setLogoutError("注销失败，会话可能仍然有效，请重试。");
      }
      return;
    }
    if (mounted.current) {
      setLogoutError(null);
      setIdentity(null);
      setPhase("anonymous");
    }
  }, []);

  return {
    phase,
    authMode,
    features,
    identity,
    bootError,
    logoutError,
    loginPending,
    loginError,
    login,
    logout,
    retryBoot,
  };
}

function loginErrorMessage(error: unknown): string {
  if (error instanceof APIError && error.status === 429) {
    return "尝试次数过多，请稍后再试。";
  }
  return "用户名或密码错误。";
}
