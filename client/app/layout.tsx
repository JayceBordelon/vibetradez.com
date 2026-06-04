import type { Metadata } from "next";
import { IBM_Plex_Mono, Lora, Plus_Jakarta_Sans } from "next/font/google";
import { ThemeProvider } from "next-themes";
import { SessionProvider } from "@/lib/session";
import "./globals.css";

const plusJakarta = Plus_Jakarta_Sans({
  subsets: ["latin"],
  variable: "--font-sans",
});

const lora = Lora({
  subsets: ["latin"],
  variable: "--font-serif",
});

const plexMono = IBM_Plex_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-mono",
});

export const metadata: Metadata = {
  metadataBase: new URL("https://vibetradez.com"),
  title: {
    default: "VibeTradez | An AI Runs a Real Brokerage Account",
    template: "%s | VibeTradez",
  },
  description:
    "A live experiment in letting one silly model run real money. Every weekday Claudia looks at the account, the news, and the market, then decides what to do: buy a stock, buy a call or a put, trim, sell, or hold cash. She holds what she believes in across days, all sized within hard caps. Watch the book live and get the daily recap, free.",
  keywords: ["AI trading", "AI portfolio manager", "stocks", "options", "Claude", "autonomous trading", "brokerage account"],
  authors: [{ name: "Jayce Bordelon", url: "https://jaycebordelon.com" }],
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "https://vibetradez.com",
    siteName: "VibeTradez",
    title: "VibeTradez | An AI Runs a Real Brokerage Account",
    description: "One silly model running one real brokerage account. Watch the book live and get the daily recap, free.",
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
    title: "VibeTradez | An AI Runs a Real Brokerage Account",
    description: "One silly model running one real brokerage account. Watch the book live and get the daily recap, free.",
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
      <body suppressHydrationWarning className={`${plusJakarta.variable} ${lora.variable} ${plexMono.variable} font-sans antialiased`}>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <SessionProvider>{children}</SessionProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
