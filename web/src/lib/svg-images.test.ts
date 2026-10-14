import { fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { collectLightboxState } from "./document-lightbox";
import { revealLazyImage } from "./render-rich-content";
import { enhanceSVGImage, getInlineSVGImage, loadSVGImage } from "./svg-images";

const markup = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 300">
  <style>text { fill: red; }</style>
  <defs><linearGradient id="paint"><stop stop-color="red"/></linearGradient></defs>
  <rect width="600" height="300" fill="url(#paint)"/>
  <text x="20" y="40">可选择的 SVG 文字</text>
</svg>`;

function fetchSVG(content = markup) {
  const fetcher = vi.fn().mockResolvedValue(new Response(content));
  vi.stubGlobal("fetch", fetcher);
  return fetcher;
}

function imageRoot(html = '<img src="/diagram.svg" alt="架构图">') {
  const root = document.createElement("div");
  root.innerHTML = html;
  document.body.append(root);
  const image = root.querySelector("img");
  if (image === null) throw new Error("missing image");
  image.dataset.m2hLightboxItem = "true";
  Object.defineProperties(image, {
    complete: { configurable: true, value: true },
    naturalWidth: { value: 600 },
    naturalHeight: { value: 300 },
  });
  return { root, image };
}

describe("native SVG images", () => {
  it("keeps text, styles, gradients and HTML labels while removing active content", async () => {
    fetchSVG(
      markup.replace(
        "</svg>",
        `<script>alert(1)</script>
      <text onload="alert(2)">safe</text>
      <a href="javascript:alert(3)"><text>link</text></a>
      <foreignObject><div xmlns="http://www.w3.org/1999/xhtml" onclick="alert(4)">HTML label</div></foreignObject>
      <set attributeName="href" to="javascript:alert(5)"/>
    </svg>`,
      ),
    );
    const svg = await loadSVGImage("/diagram.svg");
    expect(svg?.querySelector("text")?.textContent).toContain("可选择");
    expect(svg?.querySelector("style")?.textContent).toContain("fill: red");
    expect(svg?.querySelector("linearGradient")?.id).toBe("paint");
    expect(svg?.querySelector("foreignObject div")?.textContent).toBe(
      "HTML label",
    );
    expect(svg?.querySelector("script, set, [onload], [onclick]")).toBeNull();
    expect(svg?.querySelector("a")?.hasAttribute("href")).toBe(false);
  });

  it.each([
    "not SVG",
    "<svg>",
    '<html xmlns="http://www.w3.org/1999/xhtml">x</html>',
  ])("rejects malformed or non-SVG resources: %s", async (content) => {
    fetchSVG(content);
    expect(await loadSVGImage("/diagram.svg")).toBeNull();
  });

  it.each([
    ":host { position: fixed }",
    "@import '/ui/markdown.css';",
    "::slotted(*) { display: none }",
  ])("leaves host-affecting styles on the image path: %s", async (style) => {
    fetchSVG(markup.replace("text { fill: red; }", style));
    expect(await loadSVGImage("/diagram.svg")).toBeNull();
  });

  it("resolves relative embedded resources against the SVG URL", async () => {
    const fetcher = fetchSVG(
      markup.replace("</svg>", '<image href="./part.png"/></svg>'),
    );
    const svg = await loadSVGImage("https://images.example/diagrams/main.svg");
    expect(fetcher).toHaveBeenCalledWith(
      "https://images.example/diagrams/main.svg",
      { credentials: "same-origin" },
    );
    expect(svg?.querySelector("image")?.getAttribute("href")).toBe(
      "https://images.example/diagrams/part.png",
    );
    expect(svg?.querySelector("rect")?.getAttribute("fill")).toBe(
      "url(#paint)",
    );
  });

  it("keeps unavailable resources as images", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("CORS")));
    expect(await loadSVGImage("https://images.example/main.svg")).toBeNull();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("missing", { status: 404 })),
    );
    expect(await loadSVGImage("/missing.svg")).toBeNull();
    expect(await loadSVGImage("javascript:alert(1)")).toBeNull();
  });

  it("promotes a cached SVG into an isolated text-selectable tree and snapshots its markup", async () => {
    const fetcher = fetchSVG();
    const { root, image } = imageRoot();
    enhanceSVGImage(image);
    await waitFor(() => expect(image.hidden).toBe(true));
    const host = root.querySelector<HTMLElement>(".m2h-svg-image");
    expect(
      host?.firstElementChild?.shadowRoot?.querySelector("text")?.textContent,
    ).toContain("可选择");
    expect(host?.getAttribute("aria-label")).toBe("架构图");
    expect(host?.style.getPropertyValue("contain")).toBe("layout paint");
    expect(root.querySelector("svg")).toBeNull();
    expect(getInlineSVGImage(image)).not.toBeNull();
    const item = collectLightboxState(root, image)?.items[0];
    expect(item?.kind).toBe("svg");
    if (item?.kind !== "svg") throw new Error("expected vector snapshot");
    expect(item.intrinsicWidth).toBe(600);
    expect(item.intrinsicHeight).toBe(300);
    expect(item.markup).toContain("可选择");
    fireEvent.load(image);
    await Promise.resolve();
    expect(fetcher).toHaveBeenCalledTimes(1);
    root.remove();
  });

  it("preserves the original image viewport when its aspect ratio differs from the viewBox", async () => {
    fetchSVG(markup.replace("0 0 600 300", "0 0 800 200"));
    const { root, image } = imageRoot();
    enhanceSVGImage(image);
    await waitFor(() => expect(image.hidden).toBe(true));
    const item = collectLightboxState(root, image)?.items[0];
    if (item?.kind !== "svg") throw new Error("expected SVG snapshot");
    expect(item.intrinsicWidth).toBe(600);
    expect(item.intrinsicHeight).toBe(300);
    expect(item.markup).toContain('viewBox="0 0 800 200"');
    root.remove();
  });

  it("waits for lazy settlement and uses the picture's actual SVG candidate", async () => {
    const fetcher = fetchSVG();
    const { root, image } = imageRoot(
      '<picture><source srcset="/selected.svg"><img src="/fallback.png"></picture>',
    );
    Object.defineProperty(image, "currentSrc", {
      configurable: true,
      value: "http://localhost/selected.svg",
    });
    image.dataset.m2hLazyState = "pending";
    enhanceSVGImage(image);
    expect(fetcher).not.toHaveBeenCalled();
    image.dataset.m2hLazyState = "loading";
    fireEvent.load(image);
    await Promise.resolve();
    expect(fetcher).not.toHaveBeenCalled();
    image.dataset.m2hLazyState = "loaded";
    revealLazyImage(image);
    await waitFor(() => expect(image.hidden).toBe(true));
    expect(fetcher).toHaveBeenCalledWith("http://localhost/selected.svg", {
      credentials: "same-origin",
    });
    expect(image.parentElement?.tagName).toBe("PICTURE");
    expect(root.querySelector("picture + .m2h-svg-image")).not.toBeNull();
    Object.defineProperty(image, "currentSrc", {
      value: "http://localhost/selected.png",
    });
    fireEvent.load(image);
    await waitFor(() => expect(image.hidden).toBe(false));
    expect(getInlineSVGImage(image)).toBeNull();
    expect(root.querySelector(".m2h-svg-image")).toBeNull();
    root.remove();
  });

  it("ignores late downloads after document replacement or source changes", async () => {
    let finish: (response: Response) => void = () => {};
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(
        () =>
          new Promise<Response>((resolve) => {
            finish = resolve;
          }),
      ),
    );
    const { root, image } = imageRoot();
    enhanceSVGImage(image);
    root.remove();
    finish(new Response(markup));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(image.hidden).toBe(false);
    expect(getInlineSVGImage(image)).toBeNull();
  });
});
