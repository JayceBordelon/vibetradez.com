import type { Metadata } from "next";
import { Plus_Jakarta_Sans, JetBrains_Mono } from "next/font/google";
import { ThemeProvider } from "next-themes";
import "./globals.css";

const jakarta = Plus_Jakarta_Sans({
	subsets: ["latin"],
	variable: "--font-sans",
});

const jetbrains = JetBrains_Mono({
	subsets: ["latin"],
	variable: "--font-mono",
});

export const metadata: Metadata = {
	metadataBase: new URL("https://vibetradez.com"),
	title: {
		default: "VibeTradez | AI-Powered Options Picks",
		template: "%s | VibeTradez",
	},
	description:
		"Free daily ranked options picks powered by two LLMs running independently. OpenAI GPT-5.4 and Anthropic Claude Opus 4.6 each pick from the same raw sentiment, then the union is delivered before market open with end-of-day results.",
	keywords: [
		"options trading",
		"AI trading",
		"daily options picks",
		"trade alerts",
		"options analytics",
		"stock options",
		"day trading",
	],
	authors: [{ name: "Jayce Bordelon", url: "https://jaycebordelon.com" }],
	openGraph: {
		type: "website",
		locale: "en_US",
		url: "https://vibetradez.com",
		siteName: "VibeTradez",
		title: "VibeTradez | AI-Powered Options Picks",
		description:
			"Free daily ranked options picks powered by AI with real-time charts and performance analytics.",
		images: [
			{
				url: "/og",
				width: 1200,
				height: 630,
				alt: "VibeTradez",
			},
		],
	},
	twitter: {
		card: "summary_large_image",
		title: "VibeTradez | AI-Powered Options Picks",
		description:
			"Free daily ranked options picks powered by AI with real-time charts and performance analytics.",
		creator: "@JayceBordelon",
		images: ["/og"],
	},
	robots: {
		index: true,
		follow: true,
	},
};

export default function RootLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<html lang="en" suppressHydrationWarning>
			<body
				className={`${jakarta.variable} ${jetbrains.variable} font-sans antialiased`}
			>
				<ThemeProvider
					attribute="class"
					defaultTheme="system"
					enableSystem
				>
					{children}
				</ThemeProvider>
			</body>
		</html>
	);
}
