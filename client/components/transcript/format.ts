/*
Parsing and formatting helpers for the transcript ledger. Every accessor in
here is defensive: tool results come from real production runs (and seeds,
and whatever a future tool version emits), so nothing assumes a payload
parses or matches the expected shape. Readers return undefined instead of
throwing, and the renderers fall back to a generic view or the raw string.

Key spellings vary by source: our Go tool layer marshals some structs with
PascalCase field names (no json tags on Position / HistoryEntry / StanceEntry)
while the seed and the map-built results use snake_case. The pick* helpers
take a list of candidate keys so both shapes render identically.
*/

export type Dict = Record<string, unknown>;

export function asDict(v: unknown): Dict | undefined {
  if (v && typeof v === "object" && !Array.isArray(v)) return v as Dict;
  return undefined;
}

export function asArray(v: unknown): unknown[] {
  return Array.isArray(v) ? v : [];
}

export function pickRaw(o: Dict | undefined, ...keys: string[]): unknown {
  if (!o) return undefined;
  for (const k of keys) {
    if (k in o && o[k] !== undefined && o[k] !== null) return o[k];
  }
  return undefined;
}

export function pickNum(o: Dict | undefined, ...keys: string[]): number | undefined {
  const v = pickRaw(o, ...keys);
  if (typeof v === "number" && Number.isFinite(v)) return v;
  return undefined;
}

export function pickStr(o: Dict | undefined, ...keys: string[]): string | undefined {
  const v = pickRaw(o, ...keys);
  if (typeof v === "string" && v !== "") return v;
  return undefined;
}

export function pickBool(o: Dict | undefined, ...keys: string[]): boolean | undefined {
  const v = pickRaw(o, ...keys);
  if (typeof v === "boolean") return v;
  return undefined;
}

/*
ParsedResult is a tool result string decoded once for the renderers:
  - recorded: false when the call has no recorded result at all
  - empty: the result was recorded but is an empty string
  - obj / arr: the parsed JSON when it parses to an object / array
  - errorMessage: set when the payload is the {"error": ...} refusal shape
  - raw: always the original string for fallbacks and the raw toggle
*/
export interface ParsedResult {
  recorded: boolean;
  empty: boolean;
  raw: string;
  parsed?: unknown;
  obj?: Dict;
  arr?: unknown[];
  errorMessage?: string;
}

export function parseResult(raw?: string): ParsedResult {
  if (raw === undefined) return { recorded: false, empty: false, raw: "" };
  const trimmed = raw.trim();
  if (!trimmed) return { recorded: true, empty: true, raw };
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return { recorded: true, empty: false, raw };
  }
  const obj = asDict(parsed);
  const arr = Array.isArray(parsed) ? parsed : undefined;
  let errorMessage: string | undefined;
  if (obj && "error" in obj) {
    const e = obj.error;
    errorMessage = typeof e === "string" ? e : safeStringify(e);
  }
  return { recorded: true, empty: false, raw, parsed, obj, arr, errorMessage };
}

/*
resultStatus classifies a recorded result for the header pill: an
{"error": ...} payload is an error, the Anthropic-run code tools signal
failure with a non-empty error_code or a nonzero return_code, and anything
else that came back (including an empty string) is a success. Undefined when
there is no recorded result.
*/
export function resultStatus(raw?: string): "success" | "error" | undefined {
  if (raw === undefined) return undefined;
  const trimmed = raw.trim();
  if (!trimmed) return "success";
  try {
    const o = JSON.parse(trimmed);
    if (o && typeof o === "object" && !Array.isArray(o)) {
      if ("error" in o) return "error";
      const ec = (o as { error_code?: unknown }).error_code;
      if (typeof ec === "string" && ec !== "") return "error";
      const rc = (o as { return_code?: unknown }).return_code;
      if (typeof rc === "number" && rc !== 0) return "error";
    }
  } catch {
    // non-JSON results (web prose) mean the tool returned something
  }
  return "success";
}

export function safeStringify(v: unknown, indent?: number): string {
  try {
    return JSON.stringify(v, null, indent);
  } catch {
    return String(v);
  }
}

// ── numbers ──

export function fmtUsd(n: number, decimals = 2): string {
  const sign = n < 0 ? "-" : "";
  return `${sign}$${Math.abs(n).toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals })}`;
}

export function fmtSignedUsd(n: number): string {
  return `${n >= 0 ? "+" : ""}${fmtUsd(n)}`;
}

export function fmtSignedPct(n: number, decimals = 1): string {
  return `${n >= 0 ? "+" : ""}${n.toFixed(decimals)}%`;
}

export function fmtPct(n: number, decimals = 1): string {
  return `${n.toFixed(decimals)}%`;
}

export function fmtQty(n: number): string {
  return n.toLocaleString("en-US", { maximumFractionDigits: 4 });
}

// fmtCompact renders large counts and notionals compactly: 948, 18.5M, 2.9T.
export function fmtCompact(n: number): string {
  const abs = Math.abs(n);
  if (abs >= 1e12) return `${(n / 1e12).toFixed(abs < 1e13 ? 2 : 1)}T`;
  if (abs >= 1e9) return `${(n / 1e9).toFixed(abs < 1e10 ? 2 : 1)}B`;
  if (abs >= 1e6) return `${(n / 1e6).toFixed(1)}M`;
  if (abs >= 1e3) return `${(n / 1e3).toFixed(1)}k`;
  return n.toLocaleString("en-US", { maximumFractionDigits: 2 });
}

export function fmtCompactUsd(n: number): string {
  const sign = n < 0 ? "-" : "";
  return `${sign}$${fmtCompact(Math.abs(n))}`;
}

const MONTHS_SHORT = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// fmtDateShort turns "2026-07-28" into "Jul 28, 2026". Anything else passes
// through unchanged (the seed uses placeholders like "(near month)").
export function fmtDateShort(d?: string): string | undefined {
  if (!d) return undefined;
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(d.trim());
  if (!m) return d;
  const month = MONTHS_SHORT[Number(m[2]) - 1];
  if (!month) return d;
  return `${month} ${Number(m[3])}, ${m[1]}`;
}

// ── instruments ──

export interface OccParts {
  root: string;
  expiration: string; // YYYY-MM-DD
  type: "CALL" | "PUT";
  strike: number;
}

/*
parseOcc decodes an OCC option symbol ("AMD   260728C00173000") into its
parts. Returns undefined for anything that does not match, so equity tickers
and malformed symbols fall back to plain text.
*/
export function parseOcc(sym?: string): OccParts | undefined {
  if (!sym) return undefined;
  const m = /^([A-Z.]{1,6})\s*(\d{2})(\d{2})(\d{2})([CP])(\d{8})$/.exec(sym.trim().toUpperCase());
  if (!m) return undefined;
  const strike = Number(m[6]) / 1000;
  if (!Number.isFinite(strike)) return undefined;
  return {
    root: m[1],
    expiration: `20${m[2]}-${m[3]}-${m[4]}`,
    type: m[5] === "C" ? "CALL" : "PUT",
    strike,
  };
}

/*
splitSentences breaks a prose paragraph into checklist items on sentence
boundaries. write_summary stores action items as flowing prose, and the
ledger renders them as a checkable plan.
*/
export function splitSentences(s: string): string[] {
  return s
    .split(/(?<=[.!?])\s+/)
    .map((t) => t.trim())
    .filter(Boolean);
}
