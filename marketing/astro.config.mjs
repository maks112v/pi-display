import { defineConfig } from "astro/config";
import react from "@astrojs/react";
import tailwindcss from "@tailwindcss/vite";

// Labels shown above code blocks in the docs. Unlisted languages show none.
const codeLabels = { sh: "Terminal", toml: "TOML" };

export default defineConfig({
  site: "https://pi-display.maks112v.com",
  integrations: [react()],
  vite: { plugins: [tailwindcss()] },
  markdown: {
    shikiConfig: {
      theme: "vesper",
      transformers: [
        {
          // Wraps each block in a figure with a label and a copy button. The
          // click handler lives in DocsLayout.astro.
          root(root) {
            const label = codeLabels[this.options.lang] ?? "";
            return {
              type: "root",
              children: [
                {
                  type: "element",
                  tagName: "figure",
                  properties: { className: ["code-block"] },
                  children: [
                    {
                      type: "element",
                      tagName: "figcaption",
                      properties: {},
                      children: [
                        { type: "element", tagName: "span", properties: {}, children: [{ type: "text", value: label }] },
                        {
                          type: "element",
                          tagName: "button",
                          properties: { type: "button", dataCopy: "" },
                          children: [{ type: "text", value: "Copy" }],
                        },
                      ],
                    },
                    ...root.children,
                  ],
                },
              ],
            };
          },
        },
      ],
    },
  },
});
