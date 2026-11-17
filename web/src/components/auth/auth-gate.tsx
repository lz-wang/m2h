import type { ReactElement } from "react";

import { App } from "../../App";
import { useAuth } from "../../hooks/use-auth";
import { LoginPage } from "./login-page";

export interface AuthGateProps {
  // Injection seam for tests; production runs the real hook.
  useAuthHook?: typeof useAuth;
}

// AuthGate resolves the deployment's auth mode and the caller's session
// before any protected document API is touched. "booting" renders nothing:
// the reader must not mount — and fire its /api/files burst into a 401 —
// while the login state is still unknown. Rendering at the reader's own URL
// keeps the originally requested document and anchor intact through a
// login.
export function AuthGate({
  useAuthHook = useAuth,
}: AuthGateProps): ReactElement | null {
  const auth = useAuthHook();
  if (auth.phase === "booting") {
    return null;
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
    <App
      authIdentity={auth.phase === "authenticated" ? auth.identity : null}
      onLogout={auth.logout}
      annotationsAvailable={auth.features.annotations}
      tokensAvailable={auth.features.tokens}
    />
  );
}
