import { LoaderCircle, Search, SearchX, TriangleAlert } from "lucide-react";
import {
  type ComponentProps,
  type KeyboardEvent as ReactKeyboardEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import type { SearchResult } from "@/api";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import type { UseSearchState } from "@/use-search";

export interface SearchDialogProps {
  open: boolean;
  onOpenChange(open: boolean): void;
  search: UseSearchState;
  // Invoked when the reader activates a result; the parent owns navigation.
  onSelect(result: SearchResult): void;
}

// The global full-text search dialog: a transient, workspace-level overlay.
// Base UI owns focus trapping, Escape and backdrop dismissal; this component
// owns the query input, the active result and the keyboard selection
// (ArrowUp/ArrowDown/Enter) on top of the debounced search state. Results are
// ordinary buttons — natively focusable and announced — while the input keeps
// focus during arrow navigation so typing continues uninterrupted.
export function SearchDialog({
  open,
  onOpenChange,
  search,
  onSelect,
}: SearchDialogProps) {
  const [activeIndex, setActiveIndex] = useState(0);
  // The search state object is recreated every render; the reset goes through
  // a ref so the close effect keys on `open` alone instead of re-running with
  // every parent render.
  const searchRef = useRef(search);
  searchRef.current = search;

  // Closing is a full reset: the search is a transient overlay, so a stale
  // query or an in-flight request must not resurface on the next open.
  useEffect(() => {
    if (!open) {
      searchRef.current.reset();
      setActiveIndex(0);
    }
  }, [open]);

  // A new result set restarts the selection at the strongest match. The
  // adjustment happens during render (React's documented "derive state from
  // a change" pattern) so the selection can never observe an out-of-range
  // index, even when two result sets swap within one commit.
  const resultKey = search.results.map((result) => result.path).join("\n");
  const [seenResultKey, setSeenResultKey] = useState(resultKey);
  if (resultKey !== seenResultKey) {
    setSeenResultKey(resultKey);
    setActiveIndex(0);
  }

  const results = search.results;
  const queryRunes = [...search.query.trim()].length;
  const selectResult = (result: SearchResult) => {
    onSelect(result);
    onOpenChange(false);
  };

  const handleInputKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) => Math.min(results.length - 1, index + 1));
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => Math.max(0, index - 1));
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      const result = results[activeIndex];
      if (result !== undefined) {
        selectResult(result);
      }
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={undefined} showCloseButton={false}>
        <DialogTitle className="sr-only">全文搜索</DialogTitle>
        <DialogDescription className="sr-only">
          搜索当前文档集的全部内容，回车跳转到匹配章节
        </DialogDescription>
        <div className="search-dialog-input-row">
          <Search aria-hidden="true" />
          <input
            type="search"
            value={search.query}
            onChange={(event) => search.setQuery(event.target.value)}
            onKeyDown={handleInputKeyDown}
            placeholder="搜索所有文档…"
            aria-label="全文搜索"
          />
        </div>
        <div className="search-dialog-results">
          {renderBody({
            search,
            queryRunes,
            activeIndex,
            onActivate: setActiveIndex,
            onSelect: selectResult,
          })}
        </div>
      </DialogContent>
    </Dialog>
  );
}

interface SearchDialogBodyProps {
  search: UseSearchState;
  queryRunes: number;
  activeIndex: number;
  onActivate(index: number): void;
  onSelect(result: SearchResult): void;
}

function renderBody({
  search,
  queryRunes,
  activeIndex,
  onActivate,
  onSelect,
}: SearchDialogBodyProps) {
  if (search.query.trim() === "") {
    return <SearchDialogHint>输入关键词搜索当前文档集</SearchDialogHint>;
  }
  if (queryRunes < 2) {
    return <SearchDialogHint>请输入至少 2 个字符</SearchDialogHint>;
  }
  if (search.phase === "searching") {
    return (
      <SearchDialogHint aria-busy="true">
        <LoaderCircle className="is-spinning" aria-hidden="true" />
        正在搜索…
      </SearchDialogHint>
    );
  }
  if (search.phase === "error") {
    return (
      <SearchDialogHint role="alert">
        <TriangleAlert aria-hidden="true" />
        搜索失败，请重试
      </SearchDialogHint>
    );
  }
  if (search.results.length === 0) {
    return (
      <SearchDialogHint>
        <SearchX aria-hidden="true" />
        没有找到匹配文档
      </SearchDialogHint>
    );
  }
  return (
    <ul aria-label="搜索结果">
      {search.results.map((result, index) => (
        <li key={result.path}>
          {/* The accessible name mirrors the sidebar's "标题，路径" shape so
           * both navigations announce results the same way. */}
          <button
            type="button"
            aria-label={`${result.title}，${result.path}`}
            aria-current={index === activeIndex || undefined}
            data-active={index === activeIndex || undefined}
            onMouseEnter={() => onActivate(index)}
            onClick={() => onSelect(result)}
          >
            <span className="search-dialog-result-title">
              <span className="search-dialog-result-name">{result.title}</span>
              {result.heading !== undefined ? (
                <span className="search-dialog-result-heading">
                  {result.heading.text}
                </span>
              ) : null}
            </span>
            <span className="search-dialog-result-path">{result.path}</span>
            {result.snippet !== undefined ? (
              <span className="search-dialog-result-snippet">
                {result.snippet}
              </span>
            ) : null}
          </button>
        </li>
      ))}
    </ul>
  );
}

function SearchDialogHint({ children, ...props }: ComponentProps<"div">) {
  return (
    <div className="search-dialog-hint" {...props}>
      {children}
    </div>
  );
}
