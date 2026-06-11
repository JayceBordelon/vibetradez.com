import { ArrowRight, BarChart3, BookOpen, Compass } from "lucide-react";
import Link from "next/link";

import { Button } from "@/components/ui/button";

/*
The shared body of the 404 surface (just the centered content, no page shell).
It is rendered two ways: inside the app group's not-found so it inherits the
nav, footer, and ambient background like every other inner page, and inside the
root not-found (which supplies its own ambient shell) for any top-level route
that matches nothing.
*/

const SUGGESTIONS = [
  {
    href: "/dashboard",
    label: "Live Dashboard",
    description: "The live book: holdings, cash, and every move with its reasoning.",
    Icon: BarChart3,
  },
  {
    href: "/",
    label: "Home",
    description: "What VibeTradez is and how to follow along.",
    Icon: Compass,
  },
  {
    href: "/faq",
    label: "How it works",
    description: "How the model runs the account, the hard caps, and the data sources.",
    Icon: BookOpen,
  },
];

export function NotFoundContent() {
  return (
    <div className="mx-auto flex min-h-[70dvh] max-w-2xl flex-col items-center justify-center px-4 py-16 text-center sm:px-6">
      <div className="select-none font-mono text-[120px] font-extrabold leading-none tracking-tighter text-primary/15 sm:text-[160px]">404</div>

      <h1 className="term-display -mt-4 text-xl font-extrabold uppercase tracking-tight sm:text-2xl">This trade didn&apos;t fill</h1>
      <p className="mt-3 max-w-md text-sm leading-relaxed text-muted-foreground sm:text-base">
        The page you were looking for doesn&apos;t exist on VibeTradez. It may have moved, been renamed, or never existed in the first place.
      </p>

      <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
        <Button asChild>
          <Link href="/dashboard">
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
  );
}
