import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";
import { LoginPage } from "./components/auth/login-page";
import { useAuth } from "./hooks/use-auth";
import "./index.css";

// Keep the browser's scroll restoration enabled: an early release had stored
// "manual" on this session history entry. The reader itself remembers the
// exact offset per document (see lib/scroll-position.ts — the native
// restoration does not fire for this client-rendered shape), and staying out
// of the browser's way means it can still help wherever it does work. The
// HTML standard recommends picking the mode as early as possible, so this
// runs before anything renders; it lives in the module (not an inline
// script) because the page ships a strict script-src 'self' CSP.
if ("scrollRestoration" in history) {
  history.scrollRestoration = "auto";
}

const root = document.getElementById("root");

if (root === null) {
  throw new Error("m2h WebUI root element is missing");
}

// AuthGate resolves the deployment's auth mode and the caller's session
// before any protected document API is touched: the default mode renders the
// classic reader immediately, authenticated deployments render the login
// page until a session exists. Rendering at the reader's own URL keeps the
// originally requested document and anchor intact through a login.
function AuthGate() {
  const auth = useAuth();
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
    />
  );
}

createRoot(root).render(
  <StrictMode>
    <AuthGate />
  </StrictMode>,
);
