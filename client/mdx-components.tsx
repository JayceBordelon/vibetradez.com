import type { MDXComponents } from "mdx/types";

/*
Global MDX component map (required by @next/mdx in the App Router).

Element styling for legal prose comes from the .prose-terms CSS on the
article wrapper, so most elements pass through untouched. The one
override: external links get target=_blank + rel, matching the
hand-written pages they replaced. Internal/anchor links are left alone.
*/
export function useMDXComponents(components: MDXComponents): MDXComponents {
  return {
    a: ({ href, children, ...props }) => {
      const external = typeof href === "string" && /^https?:\/\//.test(href);
      return (
        <a href={href} {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})} {...props}>
          {children}
        </a>
      );
    },
    ...components,
  };
}
