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
Registry is the ordered list of all rollouts ever defined for this
service. Entries should never be removed or have their slugs renamed
(both would defeat the once-only guarantee). New rollouts go at the
END of the list.
*/
var Registry = []Rollout{
	autoExecutionLiveV1,
	claudeOnlyV2,
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
		sent, err := db.IsRolloutSent(r.Slug)
		if err != nil {
			log.Printf("rollouts: %s: check sent: %v", r.Slug, err)
			continue
		}
		if sent {
			continue
		}

		html, err := r.Render()
		if err != nil {
			log.Printf("rollouts: %s: render failed (will retry next deploy): %v", r.Slug, err)
			continue
		}
		if err := mail.SendTradeEmail(from, emails, r.Subject, html); err != nil {
			log.Printf("rollouts: %s: send failed (will retry next deploy): %v", r.Slug, err)
			continue
		}
		if err := db.MarkRolloutSent(r.Slug, len(emails)); err != nil {
			log.Printf("rollouts: %s: SENT but mark-sent failed (CRITICAL — re-deploy may resend): %v", r.Slug, err)
			continue
		}
		log.Printf("rollouts: %s sent to %d subscribers", r.Slug, len(emails))
	}
}
