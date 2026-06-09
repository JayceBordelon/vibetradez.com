## What & why

<!-- What does this change and why? Link any issue. -->

## Checks

- [ ] `cd server && gofmt -w . && go vet ./... && go build ./... && go test -race ./... && golangci-lint run ./...`
- [ ] `cd client && npx biome check --write . && npx next build`
- [ ] Local Docker stack boots cleanly and the dashboard renders seeded data

## UX/UI audit

Required for any PR touching `client/` (server-only PRs may delete this section).

- [ ] Booted the fresh seeded local stack and audited the rendered app against
      the changed code (`scripts/ui-audit/run.sh` captures for analysis, and/or
      a Playwright probe for behavior). Output is gitignored; nothing committed.
- [ ] Checked both viewports and both themes on every page the change touches
- [ ] Fixed any rendering / responsive / theme / behavior issue found, in this PR
- [ ] **Findings + fixes summarized below** (what was checked, what was found,
      what changed)
