/*
Package rollouts ships one-shot announcement emails to all active
subscribers as part of a deploy. Each rollout has a unique slug (the
source of truth — set in code, never in the DB) and a render function
producing the HTML body. On startup the runner walks the registry; any
slug not present in the sent_rollouts table gets bulk-emailed to all
active subscribers and then recorded as sent. A subsequent deploy of
the same code is a no-op for already-sent slugs.

Adding a new rollout is a code-only operation: write the template +
render function, append a new entry to the Registry below, deploy.
The first run after deploy fires it.
*/
package rollouts

import (
	"log"

	"vibetradez.com/internal/email"
	"vibetradez.com/internal/store"
)

/*
Rollout is a single one-shot announcement. Slug must be globally
unique and stable forever — once recorded as sent, the slug is the
single permission slip that prevents re-sending.
*/
type Rollout struct {
	Slug    string
	Subject string
	Render  func() (string, error)
}

/*
Registry is the ordered list of currently-pending rollouts. Once a
rollout has been sent it can be retired by removing its entry here
and deleting the file: the slug stays recorded in sent_rollouts
forever so the once-only guarantee still holds if a future deploy
ever reintroduces the slug. Retired rollouts v1..v5 each lived here
once and have all been recorded as sent on production.
*/
var Registry = []Rollout{
	executionRewriteV6,
}

/*
Run is invoked once on startup. Walks the registry and sends any
rollout whose slug isn't yet recorded in the sent_rollouts table.
Skips entirely when (a) there are no active subscribers (fresh deploy
with no signups) or (b) the runtime is using a local stub key for
Resend (avoids spamming during local dev / tests).

Errors are logged, never returned, never fatal — a broken rollout
template can't take the server down. The unsent slug remains pending
and will retry on the next deploy.
*/
func Run(db *store.Store, mail *email.Client, from string, isStubKey bool) {
	if isStubKey {
		log.Printf("rollouts: skipping (Resend key is a local stub)")
		return
	}
	emails, err := db.GetActiveEmails()
	if err != nil {
		log.Printf("rollouts: failed to load active subscribers: %v", err)
		return
	}
	if len(emails) == 0 {
		log.Printf("rollouts: no active subscribers, nothing to send")
		return
	}

	for _, r := range Registry {
		// Render BEFORE claim so a render error doesn't waste the slug.
		html, err := r.Render()
		if err != nil {
			log.Printf("rollouts: %s: render failed (will retry next deploy): %v", r.Slug, err)
			continue
		}

		// Atomic claim: exactly one concurrent process gets won=true.
		// The previous IsRolloutSent → Send → MarkSent flow let two
		// boots both pass the read check and both bulk-email every
		// subscriber before either wrote the mark.
		won, err := db.ClaimRollout(r.Slug, len(emails))
		if err != nil {
			log.Printf("rollouts: %s: claim failed (will retry next deploy): %v", r.Slug, err)
			continue
		}
		if !won {
			// Another process already claimed and is responsible for the send.
			continue
		}

		if err := mail.SendTradeEmail(from, emails, r.Subject, html); err != nil {
			// Slug is now permanently claimed but the email never landed.
			// We deliberately do NOT roll back the claim — re-opening the
			// race would risk double-mass-email on the next boot, which
			// is strictly worse than a single missed send. Operator can
			// `DELETE FROM sent_rollouts WHERE slug=...` to retry.
			log.Printf("rollouts: %s: CLAIMED but send failed — slug is now inert, delete the row to retry: %v", r.Slug, err)
			continue
		}
		log.Printf("rollouts: %s sent to %d subscribers", r.Slug, len(emails))
	}
}
