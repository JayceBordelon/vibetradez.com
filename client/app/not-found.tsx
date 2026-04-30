import { ArrowRight, BarChart3, BookOpen, Compass } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";

import { AmbientBackground } from "@/components/landing/ambient-background";
import { Button } from "@/components/ui/button";

export const metadata: Metadata = {
  title: "404 - Page not found",
  description: "The page you were looking for doesn't exist on VibeTradez.",
};

const SUGGESTIONS = [
  {
    href: "/",
    label: "Live Dashboard",
    description: "Today's picks ranked by Claude's conviction score.",
    Icon: BarChart3,
  },
  {
    href: "/history",
    label: "Historical Performance",
    description: "Equity curve, win rate, and per-day breakdown.",
    Icon: Compass,
  },
  {
    href: "/faq",
    label: "How it works",
    description: "Pipeline, data sources, scoring, and execution rules.",
    Icon: BookOpen,
  },
];

export default function NotFound() {
  return (
    <div className="relative min-h-dvh overflow-hidden bg-background text-foreground">
      <AmbientBackground density="muted" />
      <div className="relative z-10 mx-auto flex min-h-dvh max-w-2xl flex-col items-center justify-center px-4 py-16 text-center sm:px-6">
        <div className="select-none font-mono text-[120px] font-extrabold leading-none tracking-tighter text-primary/15 sm:text-[160px]">404</div>

        <h1 className="-mt-4 text-2xl font-semibold tracking-tight sm:text-3xl">This trade didn&apos;t fill</h1>
        <p className="mt-3 max-w-md text-sm leading-relaxed text-muted-foreground sm:text-base">
          The page you were looking for doesn&apos;t exist on VibeTradez. It may have moved, been renamed, or never existed in the first place.
        </p>

        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          <Button asChild>
            <Link href="/">
              Back to dashboard
              <ArrowRight className="ml-1.5 h-4 w-4" aria-hidden />
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/faq">Read the FAQ</Link>
          </Button>
        </div>

        <div className="mt-12 grid w-full grid-cols-1 gap-3 sm:grid-cols-3">
          {SUGGESTIONS.map((s) => (
            <Link key={s.href} href={s.href} className="lg-panel lg-edge-shine group flex flex-col items-start gap-2 p-4 text-left transition-transform duration-300 hover:-translate-y-0.5">
              <div className="rounded-md bg-foreground/5 p-1.5 dark:bg-white/5">
                <s.Icon className="h-4 w-4 text-muted-foreground transition-colors group-hover:text-foreground" aria-hidden />
              </div>
              <div className="text-sm font-semibold">{s.label}</div>
              <div className="text-xs leading-relaxed text-muted-foreground">{s.description}</div>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
