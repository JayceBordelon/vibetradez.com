## What & why

<!-- What does this change and why? Link any issue. -->

## Checks

- [ ] `cd server && gofmt -w . && go vet ./... && go build ./... && go test -race ./... && golangci-lint run ./...`
- [ ] `cd client && npx biome check --write . && npx next build`
- [ ] Local Docker stack boots cleanly and the dashboard renders seeded data

## UX/UI audit

Required for any PR touching `client/` (server-only PRs may delete this section).

- [ ] Regenerated the audit against the local stack (`scripts/ui-audit/run.sh`)
- [ ] Committed `docs/ui-audits/<date>/` (screenshots + `audit.md`)
- [ ] Added a row to the [audit index](../docs/ui-audits/README.md)
- [ ] Fixed any rendering / responsive / theme issue the audit surfaced
- [ ] **Embedded the audit below** (paste `audit.md`, or link it, so reviewers
      see every page at desktop + mobile, light + dark)

<!--
Paste the contents of docs/ui-audits/<date>/audit.md here, or link to it:
docs/ui-audits/<date>/audit.md
-->
