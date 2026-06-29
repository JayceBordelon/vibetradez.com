"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/*
Pagination is the shared pager for the book's list surfaces. It runs in two
modes:
  - Link mode (basePath): server-rendered lists like /closed carry the page in
    the ?page= query param, so each page is a real, shareable URL.
  - Controlled mode (onPageChange): live client lists like /holdings page in
    place without navigating, so the streamed re-pricing keeps running.
It matches the rest of the book: hairline rule on top, a muted "showing X to Y
of Z" range on the left, and pill page controls on the right where the current
page wears the mint gradient the CTAs use. Page is 1-based.
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

const cell = "inline-flex h-9 min-w-9 items-center justify-center rounded-lg px-3 text-sm font-medium tabular-nums transition-colors";
const inactiveCell = cn(cell, "border border-border text-muted-foreground hover:bg-muted hover:text-foreground");

interface PaginationProps {
  page: number;
  pageSize: number;
  totalItems: number;
  /** Link mode: each page target becomes `${basePath}?page=N`. */
  basePath?: string;
  /** Controlled mode: called with the target page; no navigation. */
  onPageChange?: (page: number) => void;
}

export function Pagination({ page, pageSize, totalItems, basePath, onPageChange }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  if (totalPages <= 1) return null;

  const from = (page - 1) * pageSize + 1;
  const to = Math.min(totalItems, page * pageSize);

  // goTo renders one page target as a <button> (controlled mode) or a <Link>
  // (link mode), so a single set of styles serves both the server list pages
  // and the live client lists.
  const goTo = (target: number, label: string, className: string, children: ReactNode, key?: number) => {
    if (onPageChange) {
      return (
        <button key={key} type="button" onClick={() => onPageChange(target)} aria-label={label} className={className}>
          {children}
        </button>
      );
    }
    const href = target <= 1 ? (basePath ?? "") : `${basePath}?page=${target}`;
    return (
      <Link key={key} href={href} aria-label={label} className={className}>
        {children}
      </Link>
    );
  };

  const arrow = (dir: "prev" | "next") => {
    const Icon = dir === "prev" ? ChevronLeft : ChevronRight;
    const target = dir === "prev" ? page - 1 : page + 1;
    const disabled = dir === "prev" ? page <= 1 : page >= totalPages;
    if (disabled) {
      return (
        <span aria-disabled className={cn(cell, "cursor-not-allowed border border-border/60 text-muted-foreground/40")}>
          <Icon className="h-4 w-4" />
        </span>
      );
    }
    return goTo(target, dir === "prev" ? "Previous page" : "Next page", inactiveCell, <Icon className="h-4 w-4" />);
  };

  return (
    <nav aria-label="Pagination" className="mt-8 flex flex-col items-center justify-between gap-4 border-t border-border/50 pt-6 sm:flex-row">
      <p className="text-xs text-muted-foreground">
        Showing <span className="font-medium text-foreground tabular-nums">{from}</span> to <span className="font-medium text-foreground tabular-nums">{to}</span> of <span className="font-medium text-foreground tabular-nums">{totalItems}</span>
      </p>
      <div className="flex items-center gap-1.5">
        {arrow("prev")}
        {pageWindow(page, totalPages).map((p, i) =>
          p === "gap" ? (
            <span key={`gap-${i}`} className={cn(cell, "text-muted-foreground/60")}>
              &hellip;
            </span>
          ) : p === page ? (
            <span key={p} aria-current="page" className={cn(cell, "bg-primary font-semibold text-primary-foreground shadow-sm")}>
              {p}
            </span>
          ) : (
            goTo(p, `Page ${p}`, inactiveCell, p, p)
          ),
        )}
        {arrow("next")}
      </div>
    </nav>
  );
}
