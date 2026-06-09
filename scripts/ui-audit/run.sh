#!/usr/bin/env bash
#
# One-shot UX/UI audit capture against the local Docker stack. The output
# is THROWAWAY analysis input for the auditing model (gitignored under
# scripts/ui-audit/out/) — screenshots are never committed; the model
# reads them, fixes what it finds, and the PR description carries the
# findings.
#
# Boots nothing on its own — assumes the local stack is already running on
# $BASE_URL (default http://localhost:3005). It:
#   1. waits for /health to be green,
#   2. resolves the dynamic routes (latest transcript date + kind, one
#      holding and one closed trade) from the live stack,
#   3. runs capture.mjs: the four theme x viewport combos capture
#      concurrently into out/<date>/.
#
# Usage:
#   cd vibetradez.com/local && docker compose -f docker-compose.local.yml \
#       -f docker-compose.local.override.yml up --build -d
#   cd ../scripts/ui-audit && npm install && ./run.sh
#
# Env overrides:
#   BASE_URL    frontend URL              (default http://localhost:3005)
#   HEALTH_URL  server health URL         (default http://localhost:8080/health)
#   PGURL       seeded Postgres conn      (default local stack on :5433)
#   AUDIT_DATE  output dir date stamp     (default: today, YYYY-MM-DD)
#   OUT_DIR     output directory          (default: ./out/<AUDIT_DATE>)
set -euo pipefail
cd "$(dirname "$0")"

BASE_URL="${BASE_URL:-http://localhost:3005}"
HEALTH_URL="${HEALTH_URL:-http://localhost:8080/health}"
PGURL="${PGURL:-postgresql://vibetradez:vibetradez@localhost:5433/vibetradez}"
AUDIT_DATE="${AUDIT_DATE:-$(date +%F)}"
OUT_DIR="${OUT_DIR:-$(pwd)/out/${AUDIT_DATE}}"

echo "==> waiting for server health at ${HEALTH_URL}"
for i in $(seq 1 40); do
  if curl -fsS "${HEALTH_URL}" >/dev/null 2>&1; then echo "    healthy"; break; fi
  if [ "$i" = 40 ]; then echo "    server never became healthy" >&2; exit 1; fi
  sleep 2
done

echo "==> resolving dynamic routes from seeded Postgres"
# psql via the running postgres container so a local psql client isn't required.
# `date` is stored as ISO text ('YYYY-MM-DD'), so max() orders it correctly and
# no to_char cast is needed. `|| true` keeps a transient miss from tripping set -e.
q() { docker exec vt-local-postgres psql -U vibetradez -d vibetradez -tAc "$1" 2>/dev/null | tr -d '[:space:]' || true; }
TX_DATE="$(q "SELECT max(date) FROM portfolio_sessions;")"
TX_KIND="portfolio"
if [ -z "${TX_DATE}" ]; then
  echo "    WARN: no portfolio_sessions rows; transcript routes will be skipped" >&2
fi
echo "    transcript date=${TX_DATE:-<none>} kind=${TX_KIND}"

# Detail views open in place via ?trade=<id>, where the id is computed
# server-side (compacted symbol), so resolve one holding and one closed
# trade through the API rather than reproducing the id scheme in SQL.
# Prefer an option holding so the audit exercises the dual-axis chart.
j() { curl -fsS -H "X-VT-Source: audit" "http://localhost:8080$1" 2>/dev/null || true; }
HOLDING_ID="$(j /api/portfolio/holdings | node -e '
  let s = ""; process.stdin.on("data", (c) => { s += c; }).on("end", () => {
    try {
      const hs = JSON.parse(s).holdings ?? [];
      const pick = hs.find((h) => h.kind === "option") ?? hs[0];
      process.stdout.write(pick ? pick.id : "");
    } catch { /* leave empty */ }
  });
')"
CLOSED_ID="$(j /api/portfolio/closed | node -e '
  let s = ""; process.stdin.on("data", (c) => { s += c; }).on("end", () => {
    try {
      const ts = JSON.parse(s).trades ?? [];
      process.stdout.write(ts.length ? ts[0].id : "");
    } catch { /* leave empty */ }
  });
')"
echo "    holding id=${HOLDING_ID:-<none>} closed id=${CLOSED_ID:-<none>}"

# Build the resolved route list as JSON for capture.mjs.
AUDIT_ROUTES="$(
  TX_DATE="${TX_DATE}" TX_KIND="${TX_KIND}" HOLDING_ID="${HOLDING_ID}" CLOSED_ID="${CLOSED_ID}" node -e '
    const d = process.env.TX_DATE, k = process.env.TX_KIND;
    const h = process.env.HOLDING_ID, c = process.env.CLOSED_ID;
    const routes = [
      { slug: "landing",     label: "Landing",           path: "/" },
      { slug: "dashboard",   label: "Dashboard",          path: "/dashboard" },
      { slug: "holdings",    label: "Holdings",           path: "/holdings" },
      h && { slug: "holding-detail", label: "Holding detail", path: `/holdings?trade=${encodeURIComponent(h)}` },
      { slug: "closed",      label: "Closed",             path: "/closed" },
      c && { slug: "closed-detail", label: "Closed trade detail", path: `/closed?trade=${encodeURIComponent(c)}` },
      d && { slug: "transcripts", label: "Transcripts index", path: `/transcripts/${d}` },
      d && { slug: "transcript",  label: "Session transcript", path: `/transcript/${d}/${k}` },
      { slug: "faq",         label: "FAQ",                path: "/faq" },
      { slug: "terms",       label: "Terms",              path: "/terms" },
      { slug: "privacy",     label: "Privacy",            path: "/privacy" },
    ].filter(Boolean);
    process.stdout.write(JSON.stringify(routes));
  '
)"

echo "==> pruning stale captures in ${OUT_DIR}"
# A re-run must not leave captures from routes that were removed or renamed
# since the last run: a stale screenshot would mislead the auditing model.
rm -f "${OUT_DIR}"/*.jpg

echo "==> capturing screenshots into ${OUT_DIR}"
BASE_URL="${BASE_URL}" OUT_DIR="${OUT_DIR}" AUDIT_ROUTES="${AUDIT_ROUTES}" node capture.mjs

echo "==> done. Captures (throwaway analysis input) in ${OUT_DIR}"
