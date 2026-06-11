import { ScrollText } from "lucide-react";
import type { Metadata } from "next";

import TermsContent from "@/content/terms.mdx";

const OG_IMAGE = "/og/terms.png";

export const metadata: Metadata = {
  title: "Terms of Service",
  description: "VibeTradez terms of service: risk disclosures and legal disclaimers for an AI that runs a single personal brokerage account.",
  openGraph: {
    title: "VibeTradez | Terms of Service",
    description: "VibeTradez terms of service: risk disclosures and legal disclaimers for an AI that runs a single personal brokerage account.",
    images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | Terms of Service",
    images: [OG_IMAGE],
  },
};

// Mirrors the section ids in content/terms.mdx so the sticky TOC anchors line up.
const sections = [
  { id: "experimental", title: "Experimental Nature of This Service" },
  { id: "not-advice", title: "Not Financial Advice" },
  { id: "risk", title: "Significant Risk Disclosure" },
  { id: "how-it-trades", title: "How the Portfolio Manager Trades" },
  { id: "data", title: "Data Sources & Accuracy" },
  { id: "warranty", title: "No Warranty & Limitation of Liability" },
  { id: "contact", title: "Contact" },
];

export default function TermsPage() {
  return (
    <div className="mx-auto max-w-6xl px-4 py-12 sm:px-6">
      <div className="mb-10 flex items-start gap-3">
        <div className="lg-control p-2">
          <ScrollText className="h-5 w-5 text-primary" />
        </div>
        <div>
          <h1 className="term-display text-xl font-extrabold uppercase tracking-tight sm:text-2xl">Terms of Service</h1>
          <p className="mt-1 text-sm text-muted-foreground">Last updated: May 2026</p>
        </div>
      </div>

      <div className="grid gap-10 lg:grid-cols-[200px_1fr]">
        {/* Sticky TOC (desktop only) */}
        <aside className="hidden lg:block">
          <nav className="sticky top-24">
            <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">On this page</div>
            <ul className="mt-3 space-y-1.5 text-sm">
              {sections.map((s, i) => (
                <li key={s.id}>
                  <a href={`#${s.id}`} className="block py-1 text-muted-foreground transition-colors hover:text-foreground">
                    {i + 1}. {s.title}
                  </a>
                </li>
              ))}
            </ul>
          </nav>
        </aside>

        {/* Long-form content, authored in content/terms.mdx */}
        <article className="prose-terms min-w-0 max-w-2xl">
          <TermsContent />
          <div className="mt-12">
            <a href="#top" className="inline-flex min-h-11 items-center text-sm text-muted-foreground underline underline-offset-2 hover:text-foreground sm:min-h-0">
              Back to top
            </a>
          </div>
        </article>
      </div>
    </div>
  );
}
