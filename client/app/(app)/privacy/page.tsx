import { Lock } from "lucide-react";
import type { Metadata } from "next";

import PrivacyContent from "@/content/privacy.mdx";

const OG_IMAGE = "/og/privacy.png";

export const metadata: Metadata = {
  title: "Privacy Policy",
  description: "VibeTradez privacy policy: what data is collected, how it's used, and which third-party processors handle it.",
  openGraph: {
    title: "VibeTradez | Privacy Policy",
    description: "VibeTradez privacy policy: what data is collected, how it's used, and which third-party processors handle it.",
    images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | Privacy Policy",
    images: [OG_IMAGE],
  },
};

// Mirrors the section ids in content/privacy.mdx so the sticky TOC anchors line up.
const sections = [
  { id: "scope", title: "Scope" },
  { id: "what-we-collect", title: "What We Collect" },
  { id: "how-we-use-it", title: "How We Use It" },
  { id: "processors", title: "Third-Party Processors" },
  { id: "no-payment-data", title: "No Payment Data" },
  { id: "retention", title: "Retention & Deletion" },
  { id: "your-rights", title: "Your Rights" },
  { id: "cookies", title: "Cookies" },
  { id: "security", title: "Security" },
  { id: "changes", title: "Changes to This Policy" },
  { id: "contact", title: "Contact" },
];

export default function PrivacyPage() {
  return (
    <div className="mx-auto max-w-6xl px-4 py-12 sm:px-6">
      <div className="mb-10 flex items-start gap-3">
        <div className="lg-control p-2">
          <Lock className="h-5 w-5 text-primary" />
        </div>
        <div>
          <h1 className="term-display text-xl font-extrabold uppercase tracking-tight sm:text-2xl">Privacy Policy</h1>
          <p className="mt-1 text-sm text-muted-foreground">Last updated: May 2026</p>
        </div>
      </div>

      <div className="grid gap-10 lg:grid-cols-[200px_1fr]">
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

        {/* Long-form content, authored in content/privacy.mdx */}
        <article className="prose-terms min-w-0 max-w-2xl">
          <PrivacyContent />
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
