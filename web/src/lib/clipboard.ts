// Shared clipboard write helper.
//
// m2h previews are typically served over plain HTTP (http://127.0.0.1:8793 or
// http://192.168.x.x:port on a LAN), which is not a secure context: the async
// Clipboard API is unavailable there, and document.execCommand("copy") invoked
// inside a user gesture remains the browser-compatible fallback. Every copy
// surface — code-block buttons, the share menu, sidebar context menus — goes
// through this one helper so none of them regresses to a
// clipboard.writeText-only implementation.
export async function copyText(value: string): Promise<boolean> {
  // navigator.clipboard is deliberately only attempted in a secure context.
  if (window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(value);
      return true;
    } catch {
      // Fall through when clipboard permission is unavailable or denied.
    }
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("aria-hidden", "true");
  textarea.style.cssText = "position:fixed;left:-9999px;top:0;opacity:0";
  document.body.append(textarea);
  textarea.select();
  try {
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    textarea.remove();
  }
}
