import Link from "next/link";

import { Separator } from "@/components/ui/separator";

export function Footer() {
  return (
    <footer className="border-t bg-card">
      <div className="mx-auto flex max-w-[1200px] flex-col gap-4 px-4 py-6 sm:px-7">
        <p className="max-w-3xl text-xs leading-relaxed text-muted-foreground">
          <strong className="text-foreground">Disclaimer:</strong> Not financial advice. Options trading involves substantial risk of loss. All P&amp;L figures are hypothetical and assume
          single-contract positions at mark prices. Past performance does not guarantee future results.
        </p>
        <div className="flex flex-col items-start justify-between gap-1 text-xs text-muted-foreground sm:flex-row sm:items-center sm:gap-3">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-0">
            <span className="inline-flex min-h-11 items-center sm:min-h-0">© {new Date().getFullYear()} VibeTradez</span>
            <Separator orientation="vertical" className="hidden h-3 sm:inline-flex" />
            <span className="inline-flex min-h-11 items-center gap-1 sm:min-h-0">
              Built by{" "}
              <a
                href="https://jaycebordelon.com"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex min-h-11 items-center font-medium text-foreground underline underline-offset-2 transition-colors hover:text-primary sm:min-h-0"
              >
                Jayce Bordelon
              </a>
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-0">
            <Link
              href="/terms"
              className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 transition-colors hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start"
            >
              Terms
            </Link>
            <Separator orientation="vertical" className="hidden h-3 sm:inline-flex" />
            <Link
              href="/faq"
              className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 transition-colors hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start"
            >
              FAQ
            </Link>
            <Separator orientation="vertical" className="hidden h-3 sm:inline-flex" />
            <a
              href="https://jaycebordelon.com"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-11 min-w-11 items-center justify-center underline underline-offset-2 transition-colors hover:text-foreground sm:min-h-0 sm:min-w-0 sm:justify-start"
            >
              jaycebordelon.com
            </a>
          </div>
        </div>
      </div>
    </footer>
  );
}
