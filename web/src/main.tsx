import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";
import "./index.css";

// The app manages scroll restoration itself (per-document sessionStorage offset
// restored once rich content settles), so the browser's own history-based
// restoration must be off. Setting this before React mounts — before the reader
// viewport exists — keeps the two mechanisms from racing and jumping on reload.
if ("scrollRestoration" in window.history) {
  window.history.scrollRestoration = "manual";
}

const root = document.getElementById("root");

if (root === null) {
  throw new Error("m2h WebUI root element is missing");
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
