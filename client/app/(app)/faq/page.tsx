import type { Metadata } from "next";

import FaqContent from "@/content/faq.mdx";

const OG_IMAGE = "/og/faq.png";

export const metadata: Metadata = {
  title: "FAQ",
  description: "Frequently asked questions about VibeTradez, including how the AI portfolio manager works, the hard caps, data sources, and performance tracking.",
  openGraph: {
    title: "VibeTradez | FAQ",
    description: "Frequently asked questions about VibeTradez, including how the AI portfolio manager works, the hard caps, data sources, and performance tracking.",
    images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | FAQ",
    images: [OG_IMAGE],
  },
};

export default function FAQPage() {
  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      {/* Header (with the question-count badge) and the Q&A accordion both
          live in FaqList, which counts the QA entries authored in faq.mdx. */}
      <FaqContent />
    </div>
  );
}
