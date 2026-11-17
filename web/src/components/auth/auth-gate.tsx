import { RefreshCw, TriangleAlert } from "lucide-react";
import type { ReactElement } from "react";

import { App } from "../../App";
import { useAuth } from "../../hooks/use-auth";
import { Button } from "../ui/button";
import { LoginPage } from "./login-page";

export interface AuthGateProps {
  // Injection seam for tests; production runs the real hook.
  useAuthHook?: typeof useAuth;
}

// AuthGate resolves the deployment's auth mode and the caller's session
// before any protected document API is touched. "booting" renders nothing:
// the reader must not mount — and fire its /api/files burst into a 401 —
// while the login state is still unknown. "error" renders a retry screen:
// an unreachable config endpoint must never be read as "no auth needed".
// Rendering at the reader's own URL keeps the originally requested
// document and anchor intact through a login.
export function AuthGate({
  useAuthHook = useAuth,
}: AuthGateProps): ReactElement | null {
  const auth = useAuthHook();
  if (auth.phase === "booting") {
    return null;
  }
  if (auth.phase === "error") {
    return (
      <section className="state-panel error-panel" role="alert">
        <TriangleAlert aria-hidden="true" />
        <p>{auth.bootError ?? "无法加载站点配置。"}</p>
        <Button variant="outline" onClick={() => void auth.retryBoot()}>
          <RefreshCw aria-hidden="true" />
          重试
        </Button>
      </section>
    );
  }
  if (auth.phase === "anonymous") {
    return (
      <LoginPage
        onLogin={auth.login}
        loginPending={auth.loginPending}
        loginError={auth.loginError}
      />
    );
  }
  return (
    <>
      {auth.logoutError !== null ? (
        <div
          className="bg-destructive/10 text-destructive fixed inset-x-0 top-0 z-50 flex items-center justify-center gap-2 px-4 py-2 text-sm"
          role="alert"
        >
          <TriangleAlert aria-hidden="true" className="size-4" />
          <span>{auth.logoutError}</span>
          <Button
            variant="outline"
            size="xs"
            onClick={() => void auth.logout()}
          >
            重试注销
          </Button>
        </div>
      ) : null}
      <App
        authIdentity={auth.phase === "authenticated" ? auth.identity : null}
        onLogout={auth.logout}
        annotationsAvailable={auth.features.annotations}
        tokensAvailable={auth.features.tokens}
      />
    </>
  );
}
