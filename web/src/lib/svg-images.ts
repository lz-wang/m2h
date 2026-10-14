import DOMPurify from "dompurify";

import { imageFormat } from "./image-metadata";

const SVG_NAMESPACE = "http://www.w3.org/2000/svg";
const inlineImages = new WeakMap<HTMLImageElement, SVGSVGElement>();
const svgPromoters = new WeakMap<HTMLImageElement, () => Promise<void>>();

// Each resource gets its own shadow tree: SVG styles and fragment identifiers
// must not collide with the article, another image, or a Lightbox snapshot.
export function mountSVGImage(host: HTMLElement, svg: SVGSVGElement): void {
  // Containment belongs to an outer element beyond the imported styles' reach:
  // fixed/absolute SVG content and even an escaped :host rule cannot cover UI.
  host.style.setProperty("contain", "layout paint", "important");
  const content = document.createElement("span");
  content.className = "m2h-svg-content";
  content.style.cssText = "display:block;width:100%;height:100%";
  host.replaceChildren(content);
  const shadow = content.attachShadow({ mode: "open" });
  const style = document.createElement("style");
  style.textContent = `
    :host { display: block; line-height: normal; }
    svg { display: block; width: 100%; height: 100%; max-width: none; }
    text, tspan, foreignObject, foreignObject * {
      user-select: text !important; cursor: text; pointer-events: auto;
    }
  `;
  const visual = svg.cloneNode(true) as SVGSVGElement;
  for (const property of ["width", "height"]) {
    visual.style.setProperty(property, "100%", "important");
    visual.style.setProperty(`max-${property}`, "none", "important");
  }
  for (const text of visual.querySelectorAll<SVGElement | HTMLElement>(
    "text, tspan, foreignObject, foreignObject *",
  )) {
    text.style.setProperty("user-select", "text", "important");
    text.style.setProperty("pointer-events", "auto", "important");
  }
  shadow.replaceChildren(style, visual);
}

export function getInlineSVGImage(
  image: HTMLImageElement,
): SVGSVGElement | null {
  return inlineImages.get(image) ?? null;
}

// Fetch the browser-selected candidate, never guess a picture/srcset winner.
// A server denying CORS or malformed SVG leaves the already loaded img intact.
export async function loadSVGImage(
  source: string,
): Promise<SVGSVGElement | null> {
  try {
    const url = new URL(source, document.baseURI);
    if (!["http:", "https:", "data:", "blob:"].includes(url.protocol)) {
      return null;
    }
    const response = await fetch(url.href, { credentials: "same-origin" });
    if (!response.ok) return null;
    const parsed = new DOMParser().parseFromString(
      await response.text(),
      "image/svg+xml",
    );
    const original = parsed.documentElement;
    if (
      original.localName !== "svg" ||
      original.namespaceURI !== SVG_NAMESPACE ||
      parsed.querySelector("parsererror") !== null
    ) {
      return null;
    }
    // Shadow styles must not style their host or import a stylesheet. These
    // unusual documents retain their original image presentation instead.
    for (const style of original.querySelectorAll("style")) {
      if (/:host|::slotted|@import/i.test(style.textContent ?? "")) return null;
    }
    const clean = DOMPurify.sanitize(original, {
      USE_PROFILES: { svg: true, svgFilters: true, html: true },
      ADD_TAGS: ["foreignObject"],
      HTML_INTEGRATION_POINTS: { foreignobject: true },
      FORBID_TAGS: ["animate", "animateMotion", "animateTransform", "set"],
      RETURN_DOM: true,
    });
    const svg =
      clean instanceof SVGSVGElement
        ? clean
        : clean instanceof Element || clean instanceof DocumentFragment
          ? clean.querySelector("svg")
          : null;
    if (!(svg instanceof SVGSVGElement)) return null;
    // Keep links visually intact, but never enable navigation that was inert
    // in the author's img. Relative resources still belong to the SVG URL.
    for (const element of svg.querySelectorAll("*")) {
      if (element.localName === "a") {
        element.removeAttribute("href");
        element.removeAttribute("xlink:href");
        element.setAttribute("tabindex", "-1");
      }
      for (const name of ["href", "xlink:href", "src"]) {
        const value = element.getAttribute(name);
        if (value && !value.startsWith("#")) {
          element.setAttribute(
            name,
            new URL(value, response.url || url.href).href,
          );
        }
      }
    }
    return svg;
  } catch {
    return null;
  }
}

// The original img remains the lazy-loading and source-selection authority.
// Only its visible presentation changes, after the selected SVG has loaded.
export function enhanceSVGImage(image: HTMLImageElement): void {
  const existing = svgPromoters.get(image);
  if (existing !== undefined) {
    void existing();
    return;
  }
  let requestedSource = "";
  const promote = async () => {
    const source = image.currentSrc || image.src;
    if (requestedSource !== "" && source !== requestedSource) {
      inlineImages.delete(image);
      image.hidden = false;
      const visual = image.closest("picture") ?? image;
      visual.parentElement?.querySelector(":scope > .m2h-svg-image")?.remove();
    }
    if (
      image.dataset.m2hFallback === "true" ||
      (image.dataset.m2hLazyState !== undefined &&
        image.dataset.m2hLazyState !== "loaded") ||
      imageFormat(image) !== "SVG"
    ) {
      return;
    }
    if (source === requestedSource) return;
    requestedSource = source;
    const svg = await loadSVGImage(source);
    if (
      svg === null ||
      !image.isConnected ||
      source !== (image.currentSrc || image.src) ||
      image.dataset.m2hFallback === "true"
    ) {
      return;
    }
    const host = document.createElement("span");
    if (!svg.hasAttribute("viewBox")) {
      svg.setAttribute(
        "viewBox",
        `0 0 ${image.naturalWidth || 300} ${image.naturalHeight || 150}`,
      );
    }
    host.className = "m2h-svg-image";
    host.setAttribute("role", "img");
    host.setAttribute("aria-label", image.alt);
    host.title = image.title;
    host.style.cssText = image.style.cssText;
    const width = image.width || image.naturalWidth || 300;
    const height = image.height || image.naturalHeight || 150;
    host.style.width ||= `${width}px`;
    host.style.aspectRatio = `${width} / ${height}`;
    mountSVGImage(host, svg);
    // A picture's img stays a direct child, preserving native source selection.
    const visual = image.closest("picture") ?? image;
    visual.parentElement?.querySelector(":scope > .m2h-svg-image")?.remove();
    visual.after(host);
    image.hidden = true;
    inlineImages.set(image, svg);
  };
  svgPromoters.set(image, promote);
  // Browsers may run microtasks between load listeners. Lazy settlement must
  // explicitly invoke us again once loaded; ordinary images use load directly.
  image.addEventListener("load", () => void promote());
  if (image.complete && image.naturalWidth > 0) void promote();
}
