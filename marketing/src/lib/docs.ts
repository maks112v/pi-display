import { getCollection, render, type CollectionEntry } from "astro:content";

// Sidebar groups, in order. Each slug is a file in src/content/docs; "index"
// is the /docs landing page.
export const nav = [
  { group: "Get started", slugs: ["index", "prepare-the-pi", "install", "run-setup"] },
  { group: "Media", slugs: ["smb", "google-drive", "local-folder", "supported-files"] },
  { group: "Configure", slugs: ["display-toml", "schedules", "stay-on", "screen-txt"] },
  { group: "Operate", slugs: ["commands", "on-site", "updating", "logs", "files", "uninstall"] },
] as const;

export type DocPage = CollectionEntry<"docs"> & { group: string; href: string };

export const docHref = (slug: string) => (slug === "index" ? "/docs" : `/docs/${slug}`);

// Every page in sidebar order. Fails the build if the nav and the content
// folder disagree, so a page can't go missing from the sidebar.
export async function docPages(): Promise<DocPage[]> {
  const entries = new Map((await getCollection("docs")).map((entry) => [entry.id, entry]));
  const pages = nav.flatMap(({ group, slugs }) =>
    slugs.map((slug) => {
      const entry = entries.get(slug);
      if (!entry) throw new Error(`docs nav lists "${slug}" but src/content/docs/${slug}.md is missing`);
      entries.delete(slug);
      return { ...entry, group, href: docHref(slug) };
    }),
  );
  if (entries.size > 0) throw new Error(`docs pages missing from the nav: ${[...entries.keys()].join(", ")}`);
  return pages;
}

export interface SearchEntry {
  title: string;
  group: string;
  href: string;
  description: string;
  headings: { text: string; slug: string }[];
  // Body as plain text, lowercased, for matching only.
  text: string;
}

// Rough Markdown-to-text for search matching: keeps words and commands, drops
// link targets, tags, and formatting characters.
const plainText = (markdown: string) =>
  markdown
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/<[^>]+>/g, " ")
    .replace(/[`*#>|_-]+/g, " ")
    .replace(/\s+/g, " ")
    .toLowerCase();

let index: Promise<SearchEntry[]> | undefined;

// The search index for the ⌘K dialog. Built once per build and shared by
// every page.
export function searchIndex() {
  index ??= docPages().then((pages) =>
    Promise.all(
      pages.map(async (page) => {
        const { headings } = await render(page);
        return {
          title: page.data.title,
          group: page.group,
          href: page.href,
          description: page.data.description,
          headings: headings.filter((h) => h.depth === 2).map(({ text, slug }) => ({ text, slug })),
          text: plainText(page.body ?? ""),
        };
      }),
    ),
  );
  return index;
}
