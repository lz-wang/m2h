import { CalendarDays, ChevronRight, Tag } from "lucide-react";

import type { FrontMatter } from "../api";

/**
 * Renders the compact date + tags summary that follows the document title in
 * the toolbar. Returns null when there is nothing to summarize so the toolbar
 * collapses to a single line for documents without metadata.
 */
export function FrontMatterSummary({
  frontmatter,
}: {
  frontmatter: FrontMatter | null;
}) {
  if (frontmatter === null) {
    return null;
  }

  const tags = frontmatter.tags ?? [];

  if (!frontmatter.date && tags.length === 0) {
    return null;
  }

  return (
    <div className="document-meta">
      {frontmatter.date ? (
        <span className="document-meta-item">
          <CalendarDays aria-hidden="true" />
          <span>{frontmatter.date}</span>
        </span>
      ) : null}

      {tags.length > 0 ? (
        <span className="document-meta-item document-meta-tags">
          <Tag aria-hidden="true" />
          <span title={tags.join(", ")}>{tags.join(" · ")}</span>
        </span>
      ) : null}
    </div>
  );
}

/**
 * Renders the collapsible frontmatter table above the document body. Every
 * value is emitted as a React text node, so YAML payloads such as
 * `author: "<script>"` can never become executable HTML.
 */
export function FrontMatterPanel({
  frontmatter,
}: {
  frontmatter: FrontMatter | null;
}) {
  if (frontmatter === null || frontmatter.entries.length === 0) {
    return null;
  }

  return (
    <details className="frontmatter-panel">
      <summary>
        <ChevronRight className="frontmatter-chevron" aria-hidden="true" />
        <span>Frontmatter</span>
        <span className="frontmatter-count">{frontmatter.entries.length}</span>
      </summary>

      <div className="frontmatter-table-wrapper">
        <table className="frontmatter-table">
          <tbody>
            {frontmatter.entries.map((entry, index) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: frontmatter keys may repeat within one document
              <tr key={`${entry.key}-${index}`}>
                <th scope="row">{entry.key}</th>
                <td>
                  <span className="frontmatter-value">{entry.value}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </details>
  );
}
