
import { cn } from "@/lib/utils";
import type { TradeKind } from "@/types/portfolio";

/*
Shared atoms for the book surfaces: the instrument chip and the detail-route
helper. The list rows themselves are the ledger tables in book-tables.tsx.
*/

type Section = "holdings" | "closed";

// The detail view is opened on the list page itself via a ?trade=<uuid> query
// param rather than a nested route, so there is no separate trade URL to 404
// on. The uuid is the trade's stable id (unique across stocks and options).
export function tradeHref(section: Section, id: string): string {
  return `/${section}?trade=${encodeURIComponent(id)}`;
}

/*
KindChip is the single instrument badge. For an option it shows the contract
side (CALL green / PUT red), since "Option" plus a separate CALL/PUT badge was
redundant; for a stock it shows a neutral "Stock". Falls back to "Option" only
if a contract type somehow isn't present.
*/
export function KindChip({ kind, contractType }: { kind: TradeKind; contractType?: string }) {
  if (kind === "option" && contractType) {
    const isCall = contractType === "CALL";
    return <span className={cn("rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider", isCall ? "border-green-border text-green" : "border-red-border text-red")}>{contractType}</span>;
  }
  return <span className="rounded bg-foreground/[0.06] px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{kind === "option" ? "Option" : "Stock"}</span>;
}

