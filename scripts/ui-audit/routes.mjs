// The user-facing routes captured by the UI audit pipeline.
//
// `path` may contain placeholders in :colon form. The runner (run.sh)
// resolves them from the live local stack before invoking capture.mjs and
// passes the concrete paths in via the AUDIT_ROUTES env var (JSON). When
// AUDIT_ROUTES is absent, this static list is used with placeholders left
// as-is (useful for a quick smoke run, though dynamic routes will 404).
//
// `slug` is the screenshot filename stem. `label` is the human title shown
// in the generated audit. `note` documents what the page is for.
//
// The /seo/[slug] routes are intentionally excluded: they are noindex
// 1200x630 OG-card capture targets, not browsable pages.

export const ROUTES = [
  { slug: "landing", label: "Landing", path: "/", note: "Marketing home / hero." },
  { slug: "dashboard", label: "Dashboard", path: "/dashboard", note: "Portfolio view: stat strip, P&L-vs-SPY decomposition chart, today's executions." },
  { slug: "holdings", label: "Holdings", path: "/holdings", note: "Current held book." },
  { slug: "holding-detail", label: "Holding detail", path: "/holdings?trade=:id", note: "One open position: stats, price/value chart, rationale." },
  { slug: "closed", label: "Closed", path: "/closed", note: "Completed round trips (paginated)." },
  { slug: "closed-detail", label: "Closed trade detail", path: "/closed?trade=:id", note: "One round trip: stats, price/value chart over the hold, rationale." },
  { slug: "transcripts", label: "Transcripts index", path: "/transcripts/:date", note: "A day's session transcript list." },
  { slug: "transcript", label: "Session transcript", path: "/transcript/:date/:kind", note: "Full tool-by-tool session reasoning for one day." },
  { slug: "faq", label: "FAQ", path: "/faq", note: "Frequently asked questions." },
  { slug: "terms", label: "Terms", path: "/terms", note: "Terms of service." },
  { slug: "privacy", label: "Privacy", path: "/privacy", note: "Privacy policy." },
];

export const VIEWPORTS = [
  { name: "desktop", width: 1440, height: 900 },
  { name: "mobile", width: 390, height: 844 },
];

export const THEMES = ["light", "dark"];
