# UX/UI audits

Every PR that changes anything a user can see gets a **model-driven UI
audit**: Claude boots the seeded local stack, looks at the rendered app
(full-page captures via `scripts/ui-audit/run.sh`, or driving Playwright
directly for behavior like animations and navigation), fixes what it finds
in the same PR, and summarizes findings + fixes in the PR description.

Screenshots are throwaway analysis input: they land in the gitignored
`scripts/ui-audit/out/` and are **never committed**. The PR description is
the audit record.

## The per-PR requirement

A PR that touches the client (anything under `client/`) must:

1. Boot the fresh seeded local stack and regenerate captures (or drive the
   live pages directly) against the changed code.
2. Actually analyze what renders: both viewports, both themes, every page
   the change touches, plus a regression glance at the rest.
3. Fix any rendering, responsive, theme, or behavior issue found, in the
   same PR.
4. Summarize what was checked, what was found, and what was fixed in the
   PR description.

Server-only PRs (no `client/` changes) are exempt. The repo's
[pull request template](../../.github/pull_request_template.md) carries
this checklist.

## Running an audit

```bash
# 1. Fresh seeded local stack (from repo root)
cd local
python3 generate-seed.py > seed.sql
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml down -v
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml up --build -d

# 2. Capture analysis input (first run downloads Chromium). The four
#    theme x viewport combos capture concurrently; output is gitignored.
cd ../scripts/ui-audit
npm install
./run.sh            # writes scripts/ui-audit/out/<today>/<page>__<viewport>__<theme>.jpg
```

For behavior that stills can't show (count-up animations, scrollspy,
live-update flows), write a short throwaway Playwright probe against the
running stack instead: drive the page, sample the DOM, assert the
behavior, then delete the probe.

The capture tool's knobs (viewports, themes, scale, output) are documented
in [`scripts/ui-audit/README.md`](../../scripts/ui-audit/README.md).

## Historical audits (frozen)

The dated folders in this directory are from the earlier convention, when
each PR committed its full screenshot set and a written `audit.md`. They
are kept as history of how the UI looked and what was found, but no new
folders are added.

| Date | Audit | Notes |
|------|-------|-------|
| 2026-06-09 (2nd) | [audit.md](2026-06-09-2/audit.md) | Dashboard P&L decomposition, executions tape, per-trade detail charts. |
| 2026-06-09 | [audit.md](2026-06-09/audit.md) | Light-theme token family (7 symptoms), mobile truncation, transcript error classification, fable-5 cost rates. |
| 2026-06-08 | [audit.md](2026-06-08/audit.md) | Baseline audit of all 9 pages. Fixed: FAQ question-count badge (10→derived). |
