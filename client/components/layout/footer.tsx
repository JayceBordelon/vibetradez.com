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
        <div className="flex flex-col items-start justify-between gap-3 text-xs text-muted-foreground sm:flex-row sm:items-center">
          <div className="flex items-center gap-2">
            <span>© {new Date().getFullYear()} VibeTradez</span>
            <Separator orientation="vertical" className="h-3" />
            <span>
              Built by{" "}
              <a href="https://jaycebordelon.com" target="_blank" rel="noopener noreferrer" className="font-medium text-foreground underline underline-offset-2 transition-colors hover:text-primary">
                Jayce Bordelon
              </a>
            </span>
          </div>
          <div className="flex items-center gap-3">
            <Link href="/terms" className="underline underline-offset-2 transition-colors hover:text-foreground">
              Terms
            </Link>
            <Separator orientation="vertical" className="h-3" />
            <Link href="/faq" className="underline underline-offset-2 transition-colors hover:text-foreground">
              FAQ
            </Link>
            <Separator orientation="vertical" className="h-3" />
            <a href="https://jaycebordelon.com" target="_blank" rel="noopener noreferrer" className="underline underline-offset-2 transition-colors hover:text-foreground">
              jaycebordelon.com
            </a>
          </div>
        </div>
      </div>
    </footer>
  );
}
