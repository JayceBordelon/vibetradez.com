import type { Metadata } from "next";

import { ModelComparisonShell } from "@/components/models/comparison-shell";

export const metadata: Metadata = {
	title: "Model Comparison",
	description:
		"Side-by-side performance of OpenAI vs Anthropic on VibeTradez' options pick rankings.",
	openGraph: {
		title: "VibeTradez | Model Comparison",
		description:
			"Side-by-side performance of OpenAI vs Anthropic on VibeTradez' options pick rankings.",
	},
};

export default function ModelsPage() {
	return <ModelComparisonShell />;
}
