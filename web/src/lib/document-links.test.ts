import { describe, expect, it } from "vitest";

import { enhanceDocumentLinks } from "./document-links";

const documentBase = "https://docs.example.com/doc/a.md";

function renderAnchors(markup: string): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = markup;
  return root;
}

function firstAnchor(root: HTMLElement): HTMLAnchorElement {
  const anchor = root.querySelector("a");
  if (!(anchor instanceof HTMLAnchorElement)) {
    throw new Error("test markup must contain an anchor");
  }
  return anchor;
}

describe("enhanceDocumentLinks", () => {
  it("opens cross-origin HTTPS links in a new browsing context with noopener", () => {
    const root = renderAnchors('<a href="https://github.com/test">GitHub</a>');

    enhanceDocumentLinks(root, documentBase);

    const anchor = firstAnchor(root);
    expect(anchor.target).toBe("_blank");
    expect(anchor.rel).toContain("noopener");
  });

  it("leaves same-origin absolute URLs in the current tab", () => {
    const root = renderAnchors(
      '<a href="https://docs.example.com/doc/b.md">b</a>',
    );

    enhanceDocumentLinks(root, documentBase);

    const anchor = firstAnchor(root);
    expect(anchor.target).toBe("");
    expect(anchor.rel).toBe("");
  });

  it("leaves site-relative document URLs untouched", () => {
    const root = renderAnchors(
      '<a href="/doc/b.md">rooted</a><a href="./b.md">relative</a>',
    );

    enhanceDocumentLinks(root, documentBase);

    for (const anchor of root.querySelectorAll<HTMLAnchorElement>("a")) {
      expect(anchor.target).toBe("");
      expect(anchor.rel).toBe("");
    }
  });

  it("leaves fragment links untouched", () => {
    const root = renderAnchors('<a href="#section">section</a>');

    enhanceDocumentLinks(root, documentBase);

    const anchor = firstAnchor(root);
    expect(anchor.target).toBe("");
    expect(anchor.rel).toBe("");
  });

  it("treats protocol-relative URLs as external", () => {
    const root = renderAnchors('<a href="//github.com/test">GitHub</a>');

    enhanceDocumentLinks(root, documentBase);

    const anchor = firstAnchor(root);
    expect(anchor.target).toBe("_blank");
    expect(anchor.rel).toContain("noopener");
  });

  it("opens a same-host link over a different scheme in a new context", () => {
    const root = renderAnchors(
      '<a href="http://docs.example.com/doc/b.md">plain HTTP</a>',
    );

    enhanceDocumentLinks(root, documentBase);

    expect(firstAnchor(root).target).toBe("_blank");
  });

  it("opens a same-host link on a different port in a new context", () => {
    const root = renderAnchors(
      '<a href="https://docs.example.com:8443/doc/b.md">port</a>',
    );

    enhanceDocumentLinks(root, documentBase);

    expect(firstAnchor(root).target).toBe("_blank");
  });

  it("leaves non-HTTP schemes such as mailto and tel to the browser", () => {
    const root = renderAnchors(
      '<a href="mailto:test@example.com">mail</a><a href="tel:123456">tel</a>',
    );

    enhanceDocumentLinks(root, documentBase);

    for (const anchor of root.querySelectorAll<HTMLAnchorElement>("a")) {
      expect(anchor.target).toBe("");
      expect(anchor.rel).toBe("");
    }
  });

  it("keeps an author-provided rel token and appends noopener", () => {
    const root = renderAnchors(
      '<a href="https://github.com/test" rel="nofollow">GitHub</a>',
    );

    enhanceDocumentLinks(root, documentBase);

    expect(firstAnchor(root).rel).toBe("nofollow noopener");
  });

  it("does not duplicate an already-present noopener", () => {
    const root = renderAnchors(
      '<a href="https://github.com/test" rel="noopener">GitHub</a>',
    );

    enhanceDocumentLinks(root, documentBase);

    expect(firstAnchor(root).rel).toBe("noopener");
  });

  it("overrides a raw-HTML author target of _self", () => {
    const root = renderAnchors(
      '<a href="https://github.com/test" target="_self">GitHub</a>',
    );

    enhanceDocumentLinks(root, documentBase);

    expect(firstAnchor(root).target).toBe("_blank");
  });

  it("skips anchors without a usable href", () => {
    const root = renderAnchors('<a href="">empty</a><a name="x">bare</a>');

    enhanceDocumentLinks(root, documentBase);

    for (const anchor of root.querySelectorAll<HTMLAnchorElement>("a")) {
      expect(anchor.target).toBe("");
      expect(anchor.rel).toBe("");
    }
  });

  it("decides against the public origin behind a reverse proxy", () => {
    // The server listens on 127.0.0.1:9527 while the reader browses
    // https://m2h.example.com; only the browser-visible origin may classify
    // links, so a same-domain absolute URL stays internal and GitHub opens
    // in a new tab.
    const root = renderAnchors(
      '<a href="https://m2h.example.com/doc/guide.md">guide</a>' +
        '<a href="https://github.com/lz-wang/m2h">m2h</a>',
    );

    enhanceDocumentLinks(root, "https://m2h.example.com/doc/README.md");

    const anchors = root.querySelectorAll<HTMLAnchorElement>("a");
    expect(anchors[0]?.target).toBe("");
    expect(anchors[1]?.target).toBe("_blank");
    expect(anchors[1]?.rel).toContain("noopener");
  });
});
