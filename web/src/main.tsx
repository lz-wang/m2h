import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { AuthGate } from "./components/auth/auth-gate";
import { PublicReader } from "./components/share/public-reader";
import { stripBasePath } from "./lib/base-path";
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

const shareID = stripBasePath(location.pathname)?.match(
  /^\/share\/([^/]+)$/,
)?.[1];

createRoot(root).render(
  <StrictMode>
    {shareID === undefined ? <AuthGate /> : <PublicReader id={shareID} />}
  </StrictMode>,
);
