# VibeTradez UX/UI Audit — 2026-06-08

Full-page screenshot audit of every user-facing page, captured against the
local Docker stack with fresh seeded data, at two viewports and both themes.

| | |
|---|---|
| **Captured** | 2026-06-08 |
| **Branch** | `ui-ux-audit-pipeline` |
| **Stack** | `local/docker-compose.local.yml` + override (frontend `:3005`), seeded via `generate-seed.py` |
| **Tool** | `scripts/ui-audit` (Playwright / headless Chromium) |
| **Viewports** | Desktop 1440×900 (full-page) · Mobile 390×844 (full-page) |
| **Themes** | Light · Dark |
| **Pages** | Landing, Dashboard, Holdings, Closed, Transcripts index, Session transcript, FAQ, Terms, Privacy |

Excluded: `/seo/[slug]` (noindex 1200×630 OG-card capture targets, not browsable).

## Findings summary

| # | Severity | Page | Finding | Status |
|---|----------|------|---------|--------|
| 1 | **Bug** | FAQ | "questions" count badge hardcoded to **10** while **12** Q&A are authored in `faq.mdx` — `QUESTION_COUNT` drifted from a "keep in sync" comment. | **Fixed** — count derived from the rendered `<QA>` children (`components/legal/faq.tsx`), so it can never drift again. Badge now reads "12 questions". |
| 2 | **Tooling bug** | Landing | First capture left the scroll-reveal chapters at `opacity:0`, leaving a large blank gap mid-page. This was a *capture artifact*, not a site defect: full-page screenshots didn't trip the `whileInView` IntersectionObservers. | **Fixed** — `capture.mjs` now scrolls the full page to fire every observer before shooting. |
| 3 | Note (by design) | Transcripts | `/transcripts/<date>` renders byte-identical to `/transcript/<date>/portfolio`. | Intentional — the short URL is an alias that renders the single portfolio transcript kind (see the route comment). No action. |
| 4 | Copy nit (deferred) | FAQ / Landing | Mixed persona naming: most surfaces say **Claudia** (the persona) but a few say **Claude** (the model) — e.g. FAQ "What tools does *Claude* actually have?" and the landing guardrails chapter. | Flagged for owner. Defensible (model vs persona) and copy is voice-sensitive, so left unchanged pending a call. |
| 5 | Content (deferred, out of scope) | OG cards (`/seo`) | The social-share cards still describe the retired three-pick options bot ("picks 3 options contracts… auto-fires all 3"), which contradicts the current autonomous-portfolio-manager architecture. | Flagged. Out of the rendered-page audit scope (OG capture targets), but worth a refresh since shared links preview stale copy. |

No layout overflow, contrast failures, broken responsive stacks, clipped text, or theme glitches were found on any audited page. Both themes render cleanly at both viewports.

---

## Landing — `/`

A long-form narrative page: glass nav over the hero, then five scroll-reveal
chapters (the setup, the daily loop, the guardrails with the caps table, the
toolbox, the receipts), testimonials, and the recap CTA. Renders fully and
correctly in both themes once the reveal observers fire (see finding #2).

| Desktop · Light | Desktop · Dark |
|---|---|
| <img src="landing__desktop__light.jpg" width="100%"> | <img src="landing__desktop__dark.jpg" width="100%"> |

| Mobile · Light | Mobile · Dark |
|---|---|
| <img src="landing__mobile__light.jpg" width="280"> | <img src="landing__mobile__dark.jpg" width="280"> |

**User perspective.** Strong narrative flow; the hero lands the premise and the
chapters pace the explanation well. The caps table reads clearly in both
themes. Chapters stack cleanly on mobile with no horizontal scroll. Only the
copy nits in findings #4/#5 (persona naming, and the unrelated stale OG cards)
are worth a follow-up.

---

## Dashboard — `/dashboard`

The portfolio view: equity / invested / settled-cash / unrealized-P&L summary,
the Account-vs-SPY equity curve, today's synopsis and tomorrow's action items,
a link into the full session, and the Explore links.

| Desktop · Light | Desktop · Dark |
|---|---|
| <img src="dashboard__desktop__light.jpg" width="100%"> | <img src="dashboard__desktop__dark.jpg" width="100%"> |

| Mobile · Light | Mobile · Dark |
|---|---|
| <img src="dashboard__mobile__light.jpg" width="280"> | <img src="dashboard__mobile__dark.jpg" width="280"> |

**User perspective.** The summary row reads at a glance and the chart is the
clear focal point — the red account line trailing the dashed SPY line tells the
story immediately, and the "Trailing SPY by 50.2 pts" badge reinforces it. Axis
labels stay legible in both themes. The four summary tiles collapse to a 2×2
grid on mobile and the chart scales down without clipping. Generous vertical
whitespace between the chart and the synopsis row is intentional breathing room,
not a layout break.

---

## Holdings — `/holdings`

Current held book: market value, unrealized P&L, position count, then options
and stocks grouped with per-row P&L.

| Desktop · Light | Desktop · Dark |
|---|---|
| <img src="holdings__desktop__light.jpg" width="100%"> | <img src="holdings__desktop__dark.jpg" width="100%"> |

| Mobile · Light | Mobile · Dark |
|---|---|
| <img src="holdings__mobile__light.jpg" width="280"> | <img src="holdings__mobile__dark.jpg" width="280"> |

**User perspective.** Clean grouped list (Options / Stocks), right-aligned
values with green/red P&L that's easy to scan. The OPTIONS vs STOCKS section
labels orient the reader. Rows stay readable on mobile with values right-aligned
and metadata wrapping rather than overflowing.

---

## Closed — `/closed`

Completed round trips with realized P&L, win rate, trade count, and pagination.

| Desktop · Light | Desktop · Dark |
|---|---|
| <img src="closed__desktop__light.jpg" width="100%"> | <img src="closed__desktop__dark.jpg" width="100%"> |

| Mobile · Light | Mobile · Dark |
|---|---|
| <img src="closed__mobile__light.jpg" width="280"> | <img src="closed__mobile__dark.jpg" width="280"> |

**User perspective.** WIN/LOSS badges and signed P&L make outcomes scannable.
Pagination ("1 2 … 10") is clear and the summary stats sit up top. On mobile the
per-trade metadata truncates with an ellipsis instead of wrapping awkwardly,
which keeps rows tidy.

---

## Transcripts index — `/transcripts/[date]`  ·  Session transcript — `/transcript/[date]/[kind]`

The day's full tool-by-tool session reasoning. The two routes render the same
content (see finding #3 — the short URL is an intentional alias).

| Desktop · Light | Desktop · Dark |
|---|---|
| <img src="transcript__desktop__light.jpg" width="100%"> | <img src="transcript__desktop__dark.jpg" width="100%"> |

| Mobile · Light | Mobile · Dark |
|---|---|
| <img src="transcript__mobile__light.jpg" width="280"> | <img src="transcript__mobile__dark.jpg" width="280"> |

**User perspective.** The header summary table plus the chronological
reasoning/tool-call log reads like a clean activity feed. Tool calls and
narration are visually distinguished. It's a long page by nature; nothing is
clipped and it stays readable on mobile.

---

## FAQ — `/faq`  *(fixed in this PR)*

Accordion of questions authored in `content/faq.mdx`.

| Desktop · Light | Desktop · Dark |
|---|---|
| <img src="faq__desktop__light.jpg" width="100%"> | <img src="faq__desktop__dark.jpg" width="100%"> |

| Mobile · Light | Mobile · Dark |
|---|---|
| <img src="faq__mobile__light.jpg" width="280"> | <img src="faq__mobile__dark.jpg" width="280"> |

**User perspective.** Clear single-column accordion that scales to mobile
without trouble. The header badge previously claimed "10 questions" against 12
visible items (finding #1) — a small but real credibility ding on a page whose
whole job is being trustworthy. Now derived from the rendered Q&A, so the badge
matches reality and won't drift when questions are added or removed.

---

## Terms — `/terms`

Legal prose with a sticky table-of-contents rail on desktop.

| Desktop · Light | Desktop · Dark |
|---|---|
| <img src="terms__desktop__light.jpg" width="100%"> | <img src="terms__desktop__dark.jpg" width="100%"> |

| Mobile · Light | Mobile · Dark |
|---|---|
| <img src="terms__mobile__light.jpg" width="280"> | <img src="terms__mobile__dark.jpg" width="280"> |

**User perspective.** The two-column layout (sticky TOC + prose) is comfortable
on desktop. The TOC is correctly `hidden lg:block`, so mobile collapses to a
single readable column with no squeeze or overflow.

---

## Privacy — `/privacy`

| Desktop · Light | Desktop · Dark |
|---|---|
| <img src="privacy__desktop__light.jpg" width="100%"> | <img src="privacy__desktop__dark.jpg" width="100%"> |

| Mobile · Light | Mobile · Dark |
|---|---|
| <img src="privacy__mobile__light.jpg" width="280"> | <img src="privacy__mobile__dark.jpg" width="280"> |

**User perspective.** Consistent with Terms — clean prose, good measure, and a
single-column mobile layout. No issues found.

---

## How to reproduce this audit

```bash
cd local
python3 generate-seed.py > seed.sql
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml down -v
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml up --build -d
cd ../scripts/ui-audit && npm install && ./run.sh
```

See [`scripts/ui-audit/README.md`](../../../scripts/ui-audit/README.md) for
knobs and [`docs/ui-audits/README.md`](../README.md) for the per-PR convention.
