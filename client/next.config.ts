import createMDX from "@next/mdx";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  // Allow .md / .mdx alongside the usual route extensions so legal + FAQ
  // content can live as Markdown rendered inside the existing app shell.
  pageExtensions: ["ts", "tsx", "md", "mdx"],
};

// No remark/rehype plugins on purpose: keeps the config Turbopack-safe
// (function-valued plugin options don't serialize across the Turbopack
// boundary). Section structure + styling come from the page shell and
// the .prose-terms CSS, not from markdown plugins.
const withMDX = createMDX({});

export default withMDX(nextConfig);
