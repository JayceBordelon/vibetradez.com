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
func Run(db *store.Store, mail *email.Client, from string, isStubKey bool, unsubURLFor func(email string) string) {
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

		res := mail.SendPersonalizedToList(from, emails, r.Subject, html, unsubURLFor)
		if res.Failed == res.Total && res.Total > 0 {
			/*
				Zero recipients landed — total failure (Resend hard-down,
				rate-limit cliff, etc.). Roll back the claim by DELETE-ing
				the sent_rollouts row so the next boot can retry. This is
				safe because nobody saw the email — no double-send risk.
			*/
			if rbErr := db.DeleteRolloutClaim(r.Slug); rbErr != nil {
				log.Printf("rollouts: %s: ZERO landed AND rollback failed (manual DELETE FROM sent_rollouts WHERE slug='%s' required): send_err=%v rollback_err=%v", r.Slug, r.Slug, res.FailureDetail(), rbErr)
			} else {
				log.Printf("rollouts: %s: ZERO landed, claim rolled back for next-boot retry (%s)", r.Slug, res.FailureDetail())
			}
			continue
		}
		if res.Failed > 0 {
			// Partial failure — some subscribers got the rollout, some
			// did not. Keep the claim (preventing double-send to the
			// successful set on next boot) and log loudly so the
			// operator can hand-retry the failed recipients.
			log.Printf("rollouts: %s: PARTIAL — %d/%d delivered, %d failed (slug remains claimed; failed addrs need manual retry): %s", r.Slug, res.Succeeded, res.Total, res.Failed, res.FailureDetail())
			continue
		}
		log.Printf("rollouts: %s sent to %d subscribers", r.Slug, res.Total)
	}
}
