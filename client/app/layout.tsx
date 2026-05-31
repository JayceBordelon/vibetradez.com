import type { Metadata } from "next";
import { Inter, JetBrains_Mono, Lora } from "next/font/google";
import { ThemeProvider } from "next-themes";
import { SessionProvider } from "@/lib/session";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-sans",
});

const lora = Lora({
  subsets: ["latin"],
  variable: "--font-serif",
});

const jetbrainsMono = JetBrains_Mono({
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
    "Live experiment in letting one silly model trade. Every morning at 9:25 ET, five minutes before the open, Claudia pulls market signals, runs the same prompt against live Schwab quotes and web search, and returns her 3 highest-conviction contracts. All 3 fire live in my brokerage at the open. Delivered free just before the bell.",
  keywords: ["options trading", "AI trading", "daily options picks", "trade alerts", "options analytics", "stock options", "day trading"],
  authors: [{ name: "Jayce Bordelon", url: "https://jaycebordelon.com" }],
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "https://vibetradez.com",
    siteName: "VibeTradez",
    title: "VibeTradez | AI-Powered Options Picks",
    description: "One silly model, live market data, conviction-scored picks. Delivered free right at the open.",
    images: [
      {
        url: "/og/landing.png",
        width: 1200,
        height: 630,
        alt: "VibeTradez",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | AI-Powered Options Picks",
    description: "One silly model, live market data, conviction-scored picks. Delivered free right at the open.",
    creator: "@JayceBordelon",
    images: ["/og/landing.png"],
  },
  robots: {
    index: true,
    follow: true,
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body suppressHydrationWarning className={`${inter.variable} ${lora.variable} ${jetbrainsMono.variable} font-sans antialiased`}>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <SessionProvider>{children}</SessionProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
