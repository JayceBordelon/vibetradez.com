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
		"Free daily ranked options picks powered by AI. 10 trades ranked by conviction, delivered before market open with end-of-day results.",
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
				url: "https://i.pinimg.com/originals/a8/1c/3b/a81c3b8dd88a4a5e34a9a601c53da921.jpg",
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
		images: ["https://i.pinimg.com/originals/a8/1c/3b/a81c3b8dd88a4a5e34a9a601c53da921.jpg"],
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
