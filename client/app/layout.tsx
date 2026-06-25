import type { Metadata } from "next";
import { IBM_Plex_Mono, Inter, Lora, Plus_Jakarta_Sans } from "next/font/google";
import { ThemeProvider } from "next-themes";
import { SessionProvider } from "@/lib/session";
import "./globals.css";

// Clean humanist sans for the whole product — calm, neutral, highly legible.
const inter = Inter({
  subsets: ["latin"],
  variable: "--font-sans",
});

// Geometric display face for big, calm headlines (.font-display / headings).
const plusJakarta = Plus_Jakarta_Sans({
  subsets: ["latin"],
  weight: ["500", "600", "700", "800"],
  variable: "--font-display",
});

// Editorial serif for the model's own narration voice in transcripts.
const lora = Lora({
  subsets: ["latin"],
  variable: "--font-serif",
});

// Monospace for tabular figures and verbatim tool/JSON output.
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
    "A live experiment in letting one silly model run real money. Every weekday Claudia looks at the account, the news, and the market, then decides what to do: buy a call or a put, trim, sell, or hold cash. She trades options only, holds what she believes in across days, all sized within hard caps. Watch the book live and get the recap, free.",
  keywords: ["AI trading", "AI portfolio manager", "stocks", "options", "Claude", "autonomous trading", "brokerage account"],
  authors: [{ name: "Jayce Bordelon", url: "https://jaycebordelon.com" }],
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "https://vibetradez.com",
    siteName: "VibeTradez",
    title: "VibeTradez | An AI Runs a Real Brokerage Account",
    description: "One silly model running one real brokerage account. Watch the book live and get the recap, free.",
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
    description: "One silly model running one real brokerage account. Watch the book live and get the recap, free.",
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
      <body suppressHydrationWarning className={`${inter.variable} ${plusJakarta.variable} ${lora.variable} ${plexMono.variable} bg-background font-sans text-foreground antialiased`}>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <SessionProvider>{children}</SessionProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
