import Link from "next/link";

import { Wordmark } from "@/components/layout/wordmark";

export function Footer() {
  return (
    <footer className="relative mt-8 border-t border-border/70">
      <div className="mx-auto flex max-w-[1200px] flex-col gap-6 px-4 py-9 sm:px-7">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex flex-col gap-3">
            <Wordmark />
            <p className="max-w-md text-xs leading-relaxed text-muted-foreground">
              An autonomous AI portfolio manager running one real brokerage account, live. Watch the book, read every decision, get the recap.
            </p>
          </div>
          <nav className="flex flex-wrap items-center gap-x-5 gap-y-1 text-sm">
            <FooterLink href="/dashboard">Dashboard</FooterLink>
            <FooterLink href="/faq">FAQ</FooterLink>
            <FooterLink href="/terms">Terms</FooterLink>
            <FooterLink href="/privacy">Privacy</FooterLink>
            <FooterLink href="https://jaycebordelon.com" external>
              Built by Jayce
            </FooterLink>
          </nav>
        </div>

        <div className="flex flex-col gap-2 border-t border-border/60 pt-5 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <p className="max-w-2xl leading-relaxed">
            <span className="font-semibold text-foreground">Not financial advice.</span> Trading involves substantial risk of loss. Every figure reflects one real brokerage account trading live with real money. Past performance does not guarantee future results.
          </p>
          <span className="shrink-0 whitespace-nowrap">© {new Date().getFullYear()} VibeTradez</span>
        </div>
      </div>
    </footer>
  );
}

function FooterLink({ href, children, external }: { href: string; children: React.ReactNode; external?: boolean }) {
  const className = "inline-flex min-h-11 items-center text-muted-foreground transition-colors hover:text-foreground sm:min-h-0";
  if (external) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" className={className}>
        {children}
      </a>
    );
  }
  return (
    <Link href={href} className={className}>
      {children}
    </Link>
  );
}
