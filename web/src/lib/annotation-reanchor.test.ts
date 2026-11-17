import { beforeEach, describe, expect, it } from "vitest";

import { reanchor } from "./annotation-reanchor";
import {
  type AnnotationSelector,
  selectorFromRange,
  TextIndex,
} from "./annotation-selector";

function mountBody(html: string): HTMLElement {
  const root = document.createElement("article");
  root.innerHTML = html;
  document.body.append(root);
  return root;
}

function selectorFor(
  index: TextIndex,
  start: number,
  end: number,
  headingId?: string,
): AnnotationSelector {
  const selector: AnnotationSelector = {
    quote: { exact: index.slice(start, end), ...index.context({ start, end }) },
    position: { start, end },
    version: 1,
  };
  if (headingId !== undefined) {
    selector.headingId = headingId;
  }
  return selector;
}

beforeEach(() => {
  document.body.replaceChildren();
});

describe("reanchor", () => {
  it("resolves in place when the position still matches", () => {
    const root = mountBody("<p>同一段未变化的正文。</p>");
    const index = new TextIndex(root);
    const result = reanchor(index, selectorFor(index, 2, 5));
    expect(result.status).toBe("resolved");
    expect(result.position).toEqual({ start: 2, end: 5 });
  });

  it("resolves after text is inserted before the quote", () => {
    const root = mountBody("<p>正文与标注目标。</p>");
    const index = new TextIndex(root);
    const selector = selectorFor(index, 3, 7); // 标注目标
    // 编辑：开头插入两个字符，引用平移到 5..9。
    const paragraph = root.querySelector("p");
    if (paragraph?.firstChild !== undefined && paragraph?.firstChild !== null) {
      paragraph.firstChild.textContent = "新增正文与标注目标。";
    }
    const updated = new TextIndex(root);
    const result = reanchor(updated, selector);
    expect(result.status).toBe("resolved");
    expect(result.position).toEqual({ start: 5, end: 9 });
    expect(result.refreshed?.position.start).toBe(5);
  });

  it("disambiguates identical quotes with prefix/suffix", () => {
    const root = mountBody("<p>前文 甲乙 后文</p><p>无关 甲乙 结尾</p>");
    const index = new TextIndex(root);
    const selector = selectorFor(index, 3, 5); // 第一段的“甲乙”
    const result = reanchor(index, {
      ...selector,
      position: { start: 999, end: 1001 },
    });
    expect(result.status).toBe("resolved");
    expect(result.position).toEqual({ start: 3, end: 5 });
  });

  it("falls back to the heading region when context repeats", () => {
    const root = mountBody(
      '<h2 id="a">甲节</h2><p>上下文 重复 上下文</p>' +
        '<h2 id="b">乙节</h2><p>上下文 重复 上下文</p>',
    );
    const index = new TextIndex(root);
    // 目标在乙节（canonical 文本中第二次出现）。
    const second = index.findAll("重复")[1] ?? 0;
    const selector = selectorFor(index, second, second + 2, "b");
    // 破坏原始位置，强制走搜索。
    selector.position = { start: 999, end: 1001 };
    const result = reanchor(index, selector);
    expect(result.status).toBe("resolved");
    expect(result.position?.start).toBe(second);
  });

  it("reports ambiguous when duplicates cannot be separated", () => {
    const root = mountBody("<p>重复 重复</p>");
    const index = new TextIndex(root);
    const selector = selectorFor(index, 0, 2);
    selector.position = { start: 999, end: 1001 };
    selector.quote.prefix = "";
    selector.quote.suffix = "";
    const result = reanchor(index, selector);
    expect(result.status).toBe("ambiguous");
    expect(result.position).toBeNull();
  });

  it("reports orphaned when the quote disappeared", () => {
    const root = mountBody("<p>全新的内容。</p>");
    const index = new TextIndex(root);
    const result = reanchor(index, {
      quote: { exact: "被删除的引用", prefix: "", suffix: "" },
      position: { start: 0, end: 7 },
      version: 1,
    });
    expect(result.status).toBe("orphaned");
  });

  it("anchors astral-plane text by code points, not UTF-16 units", () => {
    const root = mountBody("<p>前缀 \u{1F680} 尾缀</p>");
    const index = new TextIndex(root);
    const selector = selectorFor(index, 3, 4); // the rocket
    const result = reanchor(index, {
      ...selector,
      position: { start: 0, end: 1 },
    });
    expect(result.status).toBe("resolved");
    expect(result.position).toEqual({ start: 3, end: 4 });
  });

  it("disambiguates duplicate emoji quotes by context (code-point lengths)", () => {
    // The same emoji twice; only the surrounding text tells them apart.
    // The suffix probe must advance by the quote's code-point length (1),
    // not its UTF-16 length (2) — a UTF-16 step reads the wrong window
    // and both candidates collapse into "ambiguous".
    const root = mountBody("<p>甲 \u{1F600} 乙</p><p>丙 \u{1F600} 丁</p>");
    const index = new TextIndex(root);
    const second = index.findAll("\u{1F600}")[1] ?? 0;
    const selector = selectorFor(index, second, second + 1);
    selector.position = { start: 999, end: 1000 };
    const result = reanchor(index, selector);
    expect(result.status).toBe("resolved");
    expect(result.position).toEqual({ start: second, end: second + 1 });
  });

  it("re-verifies the original position by context after the document changed", () => {
    // An edit pads the first paragraph so the OTHER paragraph's identical
    // quote lands exactly on the annotation's old offset. String equality
    // at the old offset must not confirm it — the context must, and it
    // names the true (shifted) passage.
    const before = mountBody("<p>AAAA。目标句。</p><p>BBBB。目标句。</p>");
    const indexBefore = new TextIndex(before);
    const annotated = indexBefore.findAll("目标句")[1] ?? 0;
    const selector = selectorFor(indexBefore, annotated, annotated + 3);
    selector.headingId = undefined;

    before.innerHTML = "<p>PPPPPPPPPAAAA。目标句。</p><p>BBBB。目标句。</p>";
    const after = new TextIndex(before);
    // The old offset still reads the same exact string…
    expect(after.slice(annotated, annotated + 3)).toBe("目标句");
    // …yet the annotation must resolve onto the passage its context
    // names (the second one, shifted by the padding), not the old offset.
    const result = reanchor(after, selector);
    expect(result.status).toBe("resolved");
    const moved = after.findAll("目标句")[1] ?? 0;
    expect(result.position).toEqual({ start: moved, end: moved + 3 });
  });

  it("does not confirm a lone match on string equality alone after a change", () => {
    // The quote survived an edit that rewrote everything around it. The
    // single occurrence matches only by string — the stored context
    // matches nothing — and an unconfirmed association is worse than an
    // undecided one.
    const root = mountBody("<p>前文 甲乙 后文</p>");
    const index = new TextIndex(root);
    const selector = selectorFor(index, 3, 5);
    root.innerHTML = "<p>全新 甲乙 结构</p>";
    const after = new TextIndex(root);
    const result = reanchor(after, selector);
    expect(result.status).toBe("ambiguous");
    expect(result.position).toBeNull();
  });

  it("reports ambiguous after a change when duplicates share context", () => {
    const root = mountBody("<p>重复</p><p>重复</p>");
    const index = new TextIndex(root);
    const selector = selectorFor(index, 0, 2);
    // No context to separate the two occurrences, and the document
    // changed: the old position still holds the same string, but string
    // equality proves nothing — ambiguity is the honest answer.
    selector.quote.prefix = "";
    selector.quote.suffix = "";
    const result = reanchor(index, selector);
    expect(result.status).toBe("ambiguous");
    expect(result.position).toBeNull();
  });

  it("keeps the original position authoritative when the document is unchanged", () => {
    // Same-hash re-renders (theme switches, tab toggles) must not move
    // annotations: identical duplicate text at another offset is not a
    // reason to re-search when the stored position still matches.
    const root = mountBody("<p>重复</p><p>重复</p>");
    const index = new TextIndex(root);
    const second = index.findAll("重复")[1] ?? 0;
    const selector = selectorFor(index, second, second + 2);
    const result = reanchor(index, selector, true);
    expect(result.status).toBe("resolved");
    expect(result.position).toEqual({ start: second, end: second + 2 });
  });
});
