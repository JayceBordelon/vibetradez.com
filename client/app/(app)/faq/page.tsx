import { HelpCircle } from "lucide-react";
import type { Metadata } from "next";

import { Badge } from "@/components/ui/badge";
import FaqContent from "@/content/faq.mdx";

const OG_IMAGE = "/og/faq.png";

export const metadata: Metadata = {
  title: "FAQ",
  description: "Frequently asked questions about VibeTradez, including how AI trade picks work, data sources, rankings, and performance tracking.",
  openGraph: {
    title: "VibeTradez | FAQ",
    description: "Frequently asked questions about VibeTradez, including how AI trade picks work, data sources, rankings, and performance tracking.",
    images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | FAQ",
    images: [OG_IMAGE],
  },
};

// Question count for the header badge; keep in sync with content/faq.mdx.
const QUESTION_COUNT = 10;

export default function FAQPage() {
  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <div className="mb-8 flex items-start gap-3">
        <div className="lg-control p-2">
          <HelpCircle className="h-5 w-5 text-primary" />
        </div>
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Frequently Asked Questions</h1>
          <div className="mt-1 flex items-center gap-2">
            <p className="text-sm text-muted-foreground">How VibeTradez works under the hood.</p>
            <Badge variant="secondary" className="text-[11px]">
              {QUESTION_COUNT} questions
            </Badge>
          </div>
        </div>
      </div>

      {/* Q&A authored in content/faq.mdx, rendered through the accordion */}
      <FaqContent />
    </div>
  );
}
