import { beforeEach, describe, expect, it } from "vitest";

import {
  codePointsBefore,
  selectorFromRange,
  TextIndex,
  utf16OffsetAt,
} from "./annotation-selector";

function mountBody(html: string): HTMLElement {
  const root = document.createElement("article");
  root.innerHTML = html;
  document.body.append(root);
  return root;
}

const sequence = 0;

beforeEach(() => {
  document.body.replaceChildren();
});

describe("code point helpers", () => {
  it("counts code points before a UTF-16 offset, pairing surrogates", () => {
    // "a😀b": a=1 unit, emoji=2 units, b=1 unit.
    const data = "a\u{1F600}b";
    expect(codePointsBefore(data, 0)).toBe(0);
    expect(codePointsBefore(data, 1)).toBe(1);
    expect(codePointsBefore(data, 2)).toBe(2); // boundary inside the pair snaps after it
    expect(codePointsBefore(data, 3)).toBe(2);
    expect(codePointsBefore(data, 4)).toBe(3);
    expect(utf16OffsetAt(data, 0)).toBe(0);
    expect(utf16OffsetAt(data, 1)).toBe(1);
    expect(utf16OffsetAt(data, 2)).toBe(3);
    expect(utf16OffsetAt(data, 3)).toBe(4);
  });
});

describe("TextIndex", () => {
  it("concatenates content text in DOM order, skipping decorations and generated visuals", () => {
    const root = mountBody(
      '<h2 id="section">存储</h2>' +
        "<p>建议使用 SQLite 作为持久化存储。</p>" +
        '<span data-m2h-decor="code-copy">Copy</span>' +
        '<span class="katex">ignored math</span>' +
        "<svg><text>icon</text></svg>" +
        "<p>结尾。</p>",
    );
    const index = new TextIndex(root);
    expect(index.text).toBe("存储建议使用 SQLite 作为持久化存储。结尾。");
    expect(index.length).toBe(index.text.length); // all BMP here
  });

  it("measures emoji as single code points across node boundaries", () => {
    const root = mountBody("<p>a\u{1F600}b</p><p>c</p>");
    const index = new TextIndex(root);
    expect(index.length).toBe(4);
    expect(index.slice(1, 2)).toBe("\u{1F600}");
    expect(index.slice(2, 4)).toBe("bc");
  });

  it("maps DOM ranges to spans and back", () => {
    const root = mountBody("<p>第一段。</p><p>第二段正文。</p>");
    const index = new TextIndex(root);
    const first = root.querySelectorAll("p")[0]?.firstChild as Text;
    const second = root.querySelectorAll("p")[1]?.firstChild as Text;
    const range = document.createRange();
    range.setStart(first, 3);
    range.setEnd(second, 4);
    const span = index.spanFromRange(range);
    expect(span).toEqual({ start: 3, end: 8 });
    const back = index.rangeFromSpan({ start: 3, end: 8 });
    expect(back).not.toBeNull();
    expect(back?.toString()).toBe("。第二段正");
  });

  it("rejects ranges touching excluded regions", () => {
    const root = mountBody('<p>正文</p><span data-m2h-decor="x">装饰</span>');
    const index = new TextIndex(root);
    const decoration = root.querySelector("span")?.firstChild as Text;
    const range = document.createRange();
    range.selectNodeContents(decoration);
    expect(index.spanFromRange(range)).toBeNull();
  });

  it("rejects collapsed and out-of-body ranges", () => {
    const root = mountBody("<p>正文内容</p>");
    const index = new TextIndex(root);
    const text = root.querySelector("p")?.firstChild as Text;
    const collapsed = document.createRange();
    collapsed.setStart(text, 1);
    collapsed.setEnd(text, 1);
    expect(index.spanFromRange(collapsed)).toBeNull();

    const outside = document.createElement("p");
    outside.textContent = "outside";
    document.body.append(outside);
    const foreign = document.createRange();
    foreign.selectNodeContents(outside.firstChild as Text);
    expect(index.spanFromRange(foreign)).toBeNull();
  });

  it("finds every occurrence of a quote", () => {
    const root = mountBody("<p>重复 重复 重复</p>");
    const index = new TextIndex(root);
    expect(index.findAll("重复")).toEqual([0, 3, 6]);
  });
});

describe("selectorFromRange", () => {
  it("builds a versioned selector with quote, context and heading", () => {
    const root = mountBody(
      '<h2 id="storage">存储方案</h2>' +
        "<p>建议使用 SQLite 作为持久化存储，并定期备份。</p>",
    );
    const index = new TextIndex(root);
    const paragraph = root.querySelectorAll("p")[0]?.firstChild as Text;
    const range = document.createRange();
    range.setStart(paragraph, 2);
    range.setEnd(paragraph, 11);
    const selector = selectorFromRange(index, range);
    expect(selector).not.toBeNull();
    expect(selector?.version).toBe(1);
    expect(selector?.quote.exact).toBe("使用 SQLite");
    expect(selector?.quote.prefix.endsWith("建议")).toBe(true);
    expect(selector?.quote.suffix.startsWith(" 作为")).toBe(true);
    // The heading text precedes the paragraph in the canonical text.
    expect(selector?.position).toEqual({ start: 6, end: 15 });
    expect(selector?.headingId).toBe("storage");
  });

  it("returns null for whitespace-only selections", () => {
    const root = mountBody("<p>词 词</p>");
    const index = new TextIndex(root);
    const text = root.querySelector("p")?.firstChild as Text;
    const range = document.createRange();
    range.setStart(text, 1);
    range.setEnd(text, 2);
    expect(selectorFromRange(index, range)).toBeNull();
  });

  it("captures tab and block hints inside enhanced structures", () => {
    const root = mountBody(
      '<div class="m2h-tabbed-set">' +
        '<div class="m2h-tabbed-panel" data-m2h-tab-id="tab-2"><p>面板正文</p></div>' +
        "</div>" +
        "<blockquote><p>引用正文</p></blockquote>",
    );
    const index = new TextIndex(root);
    const panelText = root.querySelector(".m2h-tabbed-panel p")
      ?.firstChild as Text;
    const panelRange = document.createRange();
    panelRange.selectNodeContents(panelText);
    const panelSelector = selectorFromRange(index, panelRange);
    expect(panelSelector?.tabId).toBe("tab-2");

    const quoteText = root.querySelector("blockquote p")?.firstChild as Text;
    const quoteRange = document.createRange();
    quoteRange.selectNodeContents(quoteText);
    const quoteSelector = selectorFromRange(index, quoteRange);
    expect(quoteSelector?.blockHint).toBe("blockquote");
  });
});
