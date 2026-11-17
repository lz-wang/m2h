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
// unlocks the reader.
export type AuthPhase = "booting" | "off" | "anonymous" | "authenticated";

export interface UseAuthResult {
  phase: AuthPhase;
  authMode: AuthMode;
  identity: AuthIdentity | null;
  // Non-null while a login request is in flight; the login form disables
  // its submit button to prevent double submits.
  loginPending: boolean;
  // The last login failure as a user-facing message; cleared when the form
  // or a retry changes.
  loginError: string | null;
  login: (username: string, password: string) => Promise<boolean>;
  logout: () => Promise<void>;
}

// useAuth drives the whole authentication state machine of the WebUI: the
// boot probe (config → session), login, logout, and the mid-reading
// session-loss flip. It touches no document API — the reader mounts only
// after the phase is settled, so no protected request leaves before the
// login state is known.
export function useAuth(): UseAuthResult {
  const [phase, setPhase] = useState<AuthPhase>("booting");
  const [authMode, setAuthMode] = useState<AuthMode>("none");
  const [identity, setIdentity] = useState<AuthIdentity | null>(null);
  const [loginPending, setLoginPending] = useState(false);
  const [loginError, setLoginError] = useState<string | null>(null);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void (async () => {
      let config: SiteConfig;
      try {
        config = await fetchSiteConfig(controller.signal);
      } catch {
        // Without the capability description the safest shape is the
        // classic reader: never trap the user on a login page the server
        // never advertised.
        if (!controller.signal.aborted && mounted.current) {
          setPhase("off");
        }
        return;
      }
      if (controller.signal.aborted || !mounted.current) {
        return;
      }
      setAuthMode(config.authMode);
      if (config.authMode === "none") {
        setPhase("off");
        return;
      }
      try {
        const current = await fetchCurrentIdentity(controller.signal);
        if (controller.signal.aborted || !mounted.current) {
          return;
        }
        if (current !== null) {
          setIdentity(current);
          setPhase("authenticated");
        } else {
          setPhase("anonymous");
        }
      } catch {
        if (!controller.signal.aborted && mounted.current) {
          // The session check failing hard is an anonymous boot; the next
          // login attempt surfaces real errors.
          setPhase("anonymous");
        }
      }
    })();
    return () => {
      controller.abort();
    };
  }, []);

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
    } catch {
      // Logout is best-effort on the wire; locally the session is gone
      // either way once the reader unmounts.
    }
    if (mounted.current) {
      setIdentity(null);
      setPhase("anonymous");
    }
  }, []);

  return { phase, authMode, identity, loginPending, loginError, login, logout };
}

function loginErrorMessage(error: unknown): string {
  if (error instanceof APIError && error.status === 429) {
    return "尝试次数过多，请稍后再试。";
  }
  return "用户名或密码错误。";
}
