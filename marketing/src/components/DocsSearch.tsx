import { useEffect, useMemo, useRef, useState } from "react";
import type { SearchEntry } from "../lib/docs";

interface Result {
  href: string;
  title: string;
  // The matched section, or the page's group when the match is elsewhere.
  context: string;
}

// Every word of the query must appear somewhere on the page. Title matches
// rank first, then section headings, then body text.
function search(index: SearchEntry[], query: string): Result[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return [];
  const has = (text: string) => words.some((w) => text.toLowerCase().includes(w));

  return index
    .flatMap((entry) => {
      const haystack = `${entry.title} ${entry.description} ${entry.headings.map((h) => h.text).join(" ")} ${entry.text}`.toLowerCase();
      if (!words.every((w) => haystack.includes(w))) return [];
      const heading = entry.headings.find((h) => has(h.text));
      const score = has(entry.title) ? 3 : heading ? 2 : 1;
      return [
        {
          score,
          href: heading && !has(entry.title) ? `${entry.href}#${heading.slug}` : entry.href,
          title: entry.title,
          context: heading && !has(entry.title) ? heading.text : entry.group,
        },
      ];
    })
    .sort((a, b) => b.score - a.score)
    .slice(0, 8);
}

// The ⌘K search button and dialog. The index is built at build time, so
// searching never leaves the browser.
export default function DocsSearch({ index }: { index: SearchEntry[] }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const results = useMemo(() => search(index, query), [index, query]);

  const open = () => {
    setQuery("");
    setActive(0);
    dialog.current?.showModal();
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        if (dialog.current?.open) dialog.current.close();
        else open();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <>
      <button
        type="button"
        onClick={open}
        className="flex h-9 items-center gap-2.5 rounded-lg border border-line px-3 text-sm text-faint hover:border-edge sm:w-full sm:max-w-[400px]"
      >
        <SearchIcon />
        <span className="hidden flex-1 text-left sm:inline">Search docs</span>
        <kbd className="hidden font-mono text-xs sm:inline">⌘K</kbd>
        <span className="sr-only sm:hidden">Search docs</span>
      </button>

      <dialog
        ref={dialog}
        aria-label="Search docs"
        className="mx-auto mt-[12vh] w-[min(560px,calc(100vw-2rem))] rounded-xl border border-edge bg-bg p-0 text-fg backdrop:bg-black/70"
        onClick={(e) => e.target === dialog.current && dialog.current.close()}
      >
        <div className="flex h-12 items-center gap-3 border-b border-line px-4">
          <SearchIcon />
          <input
            autoFocus
            className="h-full flex-1 bg-transparent text-[15px] outline-none placeholder:text-faint"
            placeholder="Search docs"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                e.preventDefault();
                const step = e.key === "ArrowDown" ? 1 : -1;
                setActive((i) => (i + step + results.length) % Math.max(results.length, 1));
              } else if (e.key === "Enter" && results[active]) {
                window.location.href = results[active].href;
                dialog.current?.close();
              }
            }}
            spellCheck={false}
            autoComplete="off"
          />
          <kbd className="font-mono text-xs text-faint">esc</kbd>
        </div>
        {query.trim() !== "" && (
          <ul className="max-h-[50vh] overflow-y-auto p-2">
            {results.length === 0 ? (
              <li className="px-3 py-6 text-center text-sm text-faint">No results for "{query.trim()}"</li>
            ) : (
              results.map((result, i) => (
                <li key={result.href}>
                  <a
                    href={result.href}
                    onClick={() => dialog.current?.close()}
                    onMouseEnter={() => setActive(i)}
                    aria-selected={i === active}
                    className="flex flex-col gap-0.5 rounded-md px-3 py-2.5 aria-selected:bg-line"
                  >
                    <span className="text-sm font-medium">{result.title}</span>
                    <span className="text-[13px] text-faint">{result.context}</span>
                  </a>
                </li>
              ))
            )}
          </ul>
        )}
      </dialog>
    </>
  );
}

function SearchIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true" className="shrink-0 text-faint">
      <circle cx="6" cy="6" r="4.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M9.5 9.5 L13 13" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  );
}
