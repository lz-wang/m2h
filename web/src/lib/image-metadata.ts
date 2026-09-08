// The display format of an image, derived from its URL extension or data URL
// MIME — never from a network probe: the bytes are already fetched, so the
// URL is all the metadata that comes for free. JPEG normalizes to "JPG".
//
// Two entry points share one derivation. `imageFormat` reads a live element
// (the presentation layer's tooltip); `imageFormatFromSource` reads a bare
// source string, which is what the Lightbox snapshots carry — the snapshot
// records the image's real source (a lazy image still parked on its
// placeholder snapshots the parked original), so the format must be derived
// from that recorded source, never from the element's placeholder src.

export function imageFormat(image: HTMLImageElement): string | null {
  return imageFormatFromSource(image.currentSrc || image.src);
}

export function imageFormatFromSource(
  source: string,
  baseURL: string = window.location.href,
): string | null {
  if (source === "") {
    return null;
  }
  if (source.startsWith("data:")) {
    const comma = source.indexOf(",");
    const mime = comma === -1 ? source.slice(5) : source.slice(5, comma);
    return imageFormatFromMime(mime);
  }
  try {
    const url = new URL(source, baseURL);
    const extension = url.pathname.slice(url.pathname.lastIndexOf(".") + 1);
    return url.pathname.includes(".")
      ? imageFormatFromExtension(extension)
      : null;
  } catch {
    return null;
  }
}

function imageFormatFromExtension(extension: string): string | null {
  switch (extension.toLowerCase()) {
    case "jpg":
    case "jpeg":
    case "jpe":
      return "JPG";
    case "png":
      return "PNG";
    case "svg":
    case "svgz":
      return "SVG";
    case "webp":
      return "WEBP";
    case "gif":
      return "GIF";
    case "avif":
      return "AVIF";
    case "":
      return null;
    default:
      return extension.toUpperCase();
  }
}

function imageFormatFromMime(mime: string): string | null {
  // Data URLs may carry parameters ("image/svg+xml;charset=…", the implicit
  // ";base64") — the subtype alone decides the label.
  const subtype = mime
    .toLowerCase()
    .replace(/^image\//, "")
    .split(";")[0];
  if (subtype === "" || subtype === undefined) {
    return null;
  }
  if (subtype === "svg+xml") {
    return "SVG";
  }
  return imageFormatFromExtension(subtype.replace(/[^a-z0-9]/g, ""));
}
