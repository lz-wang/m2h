// Capture server-owned configuration before mounting document HTML. Deployment
// prefixes affect public URLs only; document keys remain workspace-relative.
export const basePath =
  document.head.querySelector<HTMLMetaElement>('meta[name="m2h-base-path"]')
    ?.content ?? "";

export function appPath(path: `/${string}`): string {
  return basePath + path;
}

export function stripBasePath(pathname: string): string | null {
  if (basePath === "") return pathname;
  if (pathname === basePath) return "/";
  if (!pathname.startsWith(`${basePath}/`)) return null;
  return pathname.slice(basePath.length);
}
