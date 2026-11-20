import { useLayoutEffect, useState } from "react";
import type { Mode, ResolvedMode } from "@/model";

function systemColorScheme(): MediaQueryList | null {
  return typeof window.matchMedia === "function"
    ? window.matchMedia("(prefers-color-scheme: dark)")
    : null;
}

function resolveMode(mode: Mode, prefersDark: boolean): ResolvedMode {
  return mode === "auto" ? (prefersDark ? "dark" : "light") : mode;
}

export function useTheme(mode: Mode): ResolvedMode {
  const [resolvedMode, setResolvedMode] = useState(() =>
    resolveMode(mode, systemColorScheme()?.matches ?? false),
  );

  useLayoutEffect(() => {
    const root = document.documentElement;
    const media = mode === "auto" ? systemColorScheme() : null;
    root.dataset.mode = mode;
    root.classList.remove("m2h-mode-light", "m2h-mode-dark", "m2h-mode-auto");
    root.classList.add(`m2h-mode-${mode}`);

    const apply = () => {
      const resolved = resolveMode(mode, media?.matches ?? false);
      root.classList.toggle("dark", resolved === "dark");
      setResolvedMode(resolved);
    };

    apply();
    media?.addEventListener("change", apply);
    return () => media?.removeEventListener("change", apply);
  }, [mode]);

  return resolvedMode;
}
