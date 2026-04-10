import type { Metadata } from "next";

import { ModelComparisonShell } from "@/components/models/comparison-shell";

const OG_IMAGE =
	"https://preview.redd.it/whats-your-favorite-trading-memes-v0-b7d4e8wf41td1.jpeg?width=640&format=pjpg&auto=webp&s=18ba5b8bb0f8dcbed9764434a54b3e5f5143486f";

export const metadata: Metadata = {
	title: "Model Comparison",
	description:
		"Side-by-side performance of OpenAI vs Anthropic on VibeTradez' options pick rankings.",
	openGraph: {
		title: "VibeTradez | Model Comparison",
		description:
			"Side-by-side performance of OpenAI vs Anthropic on VibeTradez' options pick rankings.",
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
