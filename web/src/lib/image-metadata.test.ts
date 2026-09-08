import { describe, expect, it } from "vitest";

import { imageFormat, imageFormatFromSource } from "./image-metadata";

// The derivation is URL-only by contract: no network probe, no byte sniffing.
// The table pins the labels the body presentation and the Lightbox share, so
// both can never drift apart.

describe("imageFormatFromSource", () => {
  it("labels the known raster and vector extensions", () => {
    expect(imageFormatFromSource("/a.png")).toBe("PNG");
    expect(imageFormatFromSource("/a.jpg")).toBe("JPG");
    expect(imageFormatFromSource("/a.jpeg")).toBe("JPG");
    expect(imageFormatFromSource("/a.jpe")).toBe("JPG");
    expect(imageFormatFromSource("/a.svg")).toBe("SVG");
    expect(imageFormatFromSource("/a.svgz")).toBe("SVG");
    expect(imageFormatFromSource("/a.webp")).toBe("WEBP");
    expect(imageFormatFromSource("/a.gif")).toBe("GIF");
    expect(imageFormatFromSource("/a.avif")).toBe("AVIF");
  });

  it("matches extensions case-insensitively", () => {
    expect(imageFormatFromSource("/a.PNG")).toBe("PNG");
    expect(imageFormatFromSource("/a.Jpg")).toBe("JPG");
  });

  it("passes unknown extensions through uppercased", () => {
    expect(imageFormatFromSource("/a.heic")).toBe("HEIC");
    expect(imageFormatFromSource("/a.avifs")).toBe("AVIFS");
  });

  it("reads no format from a source without a URL path extension", () => {
    expect(imageFormatFromSource("/render")).toBeNull();
    expect(imageFormatFromSource("")).toBeNull();
  });

  it("reads the format from a data URL's MIME subtype", () => {
    expect(imageFormatFromSource("data:image/png;base64,AAAA")).toBe("PNG");
    expect(
      imageFormatFromSource("data:image/svg+xml;charset=utf-8,%3Csvg"),
    ).toBe("SVG");
    expect(imageFormatFromSource("data:;base64,AAAA")).toBeNull();
  });

  it("resolves relative sources against the given base URL", () => {
    expect(
      imageFormatFromSource("images/a.png", "https://example.invalid/docs/"),
    ).toBe("PNG");
    // An absolute source wins over the base it is resolved against.
    expect(
      imageFormatFromSource(
        "https://cdn.example.invalid/a.webp",
        "https://example.invalid/docs/",
      ),
    ).toBe("WEBP");
  });

  it("answers null for a source that is not a URL", () => {
    expect(imageFormatFromSource("http://[invalid", "https://ok/")).toBeNull();
  });
});

describe("imageFormat", () => {
  it("prefers currentSrc over the src attribute", () => {
    const image = document.createElement("img");
    image.src = "/fallback.png";
    Object.defineProperty(image, "currentSrc", { value: "/winner.jpg" });
    expect(imageFormat(image)).toBe("JPG");
  });

  it("falls back to the src attribute when no source was selected", () => {
    const image = document.createElement("img");
    image.src = "/a.avif";
    expect(imageFormat(image)).toBe("AVIF");
  });
});
