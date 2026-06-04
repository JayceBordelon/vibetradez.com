import { ChevronLeft, ChevronRight } from "lucide-react";
import Link from "next/link";

import { cn } from "@/lib/utils";

/*
Pagination is a server-renderable pager (plain links, no client state) for the
list surfaces like /closed. It matches the rest of the book: hairline rule on
top, a muted "showing X to Y of Z" range on the left, and pill page controls
on the right where the current page wears the mint gradient the CTAs use.
Page is 1-based and carried in the ?page= query param.
*/

// pageWindow returns the pages to render, collapsing long runs to a "gap"
// sentinel so the control stays compact (first, last, and current +/- 1).
function pageWindow(current: number, total: number): (number | "gap")[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const out: (number | "gap")[] = [1];
  const lo = Math.max(2, current - 1);
  const hi = Math.min(total - 1, current + 1);
  if (lo > 2) out.push("gap");
  for (let p = lo; p <= hi; p++) out.push(p);
  if (hi < total - 1) out.push("gap");
  out.push(total);
  return out;
}

const cell = "inline-flex h-9 min-w-9 items-center justify-center rounded-full px-3 text-sm font-medium transition-colors";

function Arrow({ href, dir, disabled }: { href: string; dir: "prev" | "next"; disabled: boolean }) {
  const Icon = dir === "prev" ? ChevronLeft : ChevronRight;
  if (disabled) {
    return (
      <span aria-disabled className={cn(cell, "cursor-not-allowed border border-border/60 text-muted-foreground/40")}>
        <Icon className="h-4 w-4" />
      </span>
    );
  }
  return (
    <Link href={href} aria-label={dir === "prev" ? "Previous page" : "Next page"} className={cn(cell, "border border-border text-muted-foreground hover:bg-muted hover:text-foreground")}>
      <Icon className="h-4 w-4" />
    </Link>
  );
}

export function Pagination({ basePath, page, pageSize, totalItems }: { basePath: string; page: number; pageSize: number; totalItems: number }) {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  if (totalPages <= 1) return null;

  const href = (p: number) => (p <= 1 ? basePath : `${basePath}?page=${p}`);
  const from = (page - 1) * pageSize + 1;
  const to = Math.min(totalItems, page * pageSize);

  return (
    <nav aria-label="Pagination" className="mt-8 flex flex-col items-center justify-between gap-4 border-t border-border/50 pt-6 sm:flex-row">
      <p className="text-xs text-muted-foreground">
        Showing <span className="font-medium text-foreground tabular-nums">{from}</span> to <span className="font-medium text-foreground tabular-nums">{to}</span> of <span className="font-medium text-foreground tabular-nums">{totalItems}</span>
      </p>
      <div className="flex items-center gap-1.5">
        <Arrow href={href(page - 1)} dir="prev" disabled={page <= 1} />
        {pageWindow(page, totalPages).map((p, i) =>
          p === "gap" ? (
            <span key={`gap-${i}`} className={cn(cell, "text-muted-foreground/60")}>
              &hellip;
            </span>
          ) : p === page ? (
            <span key={p} aria-current="page" className={cn(cell, "bg-gradient-brand font-semibold text-primary-foreground")}>
              {p}
            </span>
          ) : (
            <Link key={p} href={href(p)} className={cn(cell, "border border-border text-muted-foreground hover:bg-muted hover:text-foreground")}>
              {p}
            </Link>
          ),
        )}
        <Arrow href={href(page + 1)} dir="next" disabled={page >= totalPages} />
      </div>
    </nav>
  );
}
