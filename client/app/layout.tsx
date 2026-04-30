import type { Metadata } from "next";
import { JetBrains_Mono, Plus_Jakarta_Sans } from "next/font/google";
import { ThemeProvider } from "next-themes";
import { SessionProvider } from "@/lib/session";
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
    "Live experiment in letting one silly model trade. Every morning Claude pulls market signals, runs the same prompt against Schwab quotes and web search, and ranks its top 10 contracts with conviction scores. Delivered free before market open.",
  keywords: ["options trading", "AI trading", "daily options picks", "trade alerts", "options analytics", "stock options", "day trading"],
  authors: [{ name: "Jayce Bordelon", url: "https://jaycebordelon.com" }],
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "https://vibetradez.com",
    siteName: "VibeTradez",
    title: "VibeTradez | AI-Powered Options Picks",
    description: "One silly model, live market data, conviction-scored picks. Delivered free before the opening bell.",
    images: [
      {
        url: "/opengraph-image",
        width: 1200,
        height: 630,
        alt: "VibeTradez",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | AI-Powered Options Picks",
    description: "One silly model, live market data, conviction-scored picks. Delivered free before the opening bell.",
    creator: "@JayceBordelon",
    images: ["/opengraph-image"],
  },
  robots: {
    index: true,
    follow: true,
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${jakarta.variable} ${jetbrains.variable} font-sans antialiased`}>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <SessionProvider>{children}</SessionProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
