# UX/UI audits

Every PR that changes anything a user can see ships with a **UX/UI audit**: a
full-page screenshot of each page at desktop and mobile, in light and dark, with
a written per-page analysis from a user's perspective and any rendering bug it
surfaced fixed in the same PR.

The audits live here, one dated folder per run:

```
docs/ui-audits/
├── README.md            ← this file
└── <YYYY-MM-DD>/
    ├── audit.md         ← the written audit, with every screenshot embedded
    └── <page>__<viewport>__<theme>.jpg   ← e.g. dashboard__mobile__dark.jpg
```

## Audit index

| Date | Audit | Notes |
|------|-------|-------|
| 2026-06-08 | [audit.md](2026-06-08/audit.md) | Baseline audit of all 9 pages. Fixed: FAQ question-count badge (10→derived). |

## The per-PR requirement

A PR that touches the client (anything under `client/`) **must**:

1. Regenerate the audit against the local stack (instructions below).
2. Commit the dated `docs/ui-audits/<date>/` folder.
3. Add a row to the index table above.
4. **Embed the audit in the PR description** — paste the contents of
   `audit.md`, or link to it, so reviewers see the rendered pages and the
   per-page analysis without leaving the PR.
5. Fix any rendering messiness / overflow / responsive break / theme glitch the
   audit surfaces, in the same PR.

Server-only PRs (no `client/` changes) are exempt.

The repo's [pull request template](../../.github/pull_request_template.md)
carries this checklist.

## Generating an audit

```bash
# 1. Fresh seeded local stack (from repo root)
cd local
python3 generate-seed.py > seed.sql
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml down -v
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml up --build -d

# 2. Capture (first run downloads Chromium)
cd ../scripts/ui-audit
npm install
./run.sh            # writes docs/ui-audits/<today>/<page>__<viewport>__<theme>.jpg

# 3. Write docs/ui-audits/<today>/audit.md (follow the prior audit's structure),
#    add the index row above, fix anything broken, then open the PR with the
#    audit embedded in the description.
```

The capture tool (viewports, themes, knobs, file naming) is documented in
[`scripts/ui-audit/README.md`](../../scripts/ui-audit/README.md).

## Conventions

- **Format:** JPEG, full-page, 1× scale. Keeps each audit ~10 MB so it can be
  committed per PR without bloating history. Pixel-peeping a specific glitch?
  Re-run one route with `SCALE=2`.
- **Filenames:** `<page-slug>__<viewport>__<theme>.jpg`, viewport ∈
  {`desktop`, `mobile`}, theme ∈ {`light`, `dark`}.
- **Date folders** are kept, not overwritten — the history of how the UI looked
  over time is part of the value.
