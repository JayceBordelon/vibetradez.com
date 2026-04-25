import type { Metadata } from "next";

import { ModelComparisonShell } from "@/components/models/comparison-shell";

const OG_IMAGE = "/og";

export const metadata: Metadata = {
  title: "Model Comparison",
  description: "Side-by-side performance of ChatGPT vs Claude on VibeTradez' options pick rankings.",
  openGraph: {
    title: "VibeTradez | Model Comparison",
    description: "Side-by-side performance of ChatGPT vs Claude on VibeTradez' options pick rankings.",
    images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | Model Comparison",
    images: [OG_IMAGE],
  },
};

export default function ModelsPage() {
  return <ModelComparisonShell />;
}
