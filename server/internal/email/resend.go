package email

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/resend/resend-go/v2"
)

type Client struct {
	client *resend.Client
}

func NewClient(apiKey string) *Client {
	/*
		Override the SDK's default 60s HTTP timeout. A single hung
		Resend call would block the entire morning cron for a minute
		per stuck recipient on the per-recipient SendPersonalizedToList
		path. 15s matches the schwab + anthropic client budgets and is
		well above Resend's typical sub-second response.
	*/
	httpClient := &http.Client{Timeout: 15 * time.Second}
	return &Client{
		client: resend.NewCustomClient(httpClient, apiKey),
	}
}

/*
perRecipientSendSpacing is the minimum delay between Resend calls in
SendPersonalizedToList. Resend Pro is 10 req/s; 200ms = 5 req/s,
under cap with headroom. Without spacing, blasting the model-authored
recap (or a one-time product update) to a 100-subscriber list burns
through the rate limit in <10s and every call after that returns
429 until the bucket refills.
*/
const perRecipientSendSpacing = 200 * time.Millisecond

func (c *Client) SendTradeEmail(from string, to []string, subject, htmlContent string) error {
	params := &resend.SendEmailRequest{
		From:    from,
		To:      to,
		Subject: subject,
		Html:    htmlContent,
	}

	_, err := c.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

/*
SendPersonalizedToList sends one Resend call per recipient,
substituting the literal placeholder "@@VT_UNSUBSCRIBE_URL@@" in
the HTML body with a per-recipient signed unsubscribe link from
unsubURLFor(email).

The previous SendTradeEmail bulk path sent identical HTML to every
recipient — fine for the trading content but a non-starter for
CAN-SPAM compliant per-recipient unsubscribe tokens. This wrapper
preserves the simple "render once, send many" calling convention
while making each delivery carry the recipient's own one-click
unsub link.

Errors from individual sends are logged-not-fatal: if recipient 7
of 10 errors, recipients 8-10 still get their email. The aggregate
error reports the count of failures so the caller can surface a
partial-send warning to the operator without abandoning the batch.
*/
/*
PersonalizedResult reports the outcome of a per-recipient batch so
the caller can decide between "retry the whole rollout" (zero
landed) and "log and move on" (some landed). Sensitive details
(per-email failure reasons) stay in failureDetail for caller-side
log redaction; the public counts are safe to surface.
*/
type PersonalizedResult struct {
	Total         int
	Succeeded     int
	Failed        int
	failureDetail string // unexported so callers must use FailureDetail() and decide to log it
}

func (r PersonalizedResult) FailureDetail() string { return r.failureDetail }

func (c *Client) SendPersonalizedToList(from string, to []string, subject, htmlContent string, unsubURLFor func(email string) string) PersonalizedResult {
	const placeholder = "@@VT_UNSUBSCRIBE_URL@@"
	result := PersonalizedResult{Total: len(to)}
	var failures []string
	for i, addr := range to {
		// Resend Pro is 10 req/s. Spacing between sends keeps us under
		// cap so a long list doesn't trip 429s mid-batch. First send
		// is unspaced; only subsequent sends sleep.
		if i > 0 {
			time.Sleep(perRecipientSendSpacing)
		}
		unsubURL := unsubURLFor(addr)
		body := strings.ReplaceAll(htmlContent, placeholder, unsubURL)
		params := &resend.SendEmailRequest{
			From:    from,
			To:      []string{addr},
			Subject: subject,
			Html:    body,
			// RFC 8058 one-click unsubscribe. Gmail's bulk-sender rules
			// require these headers since Feb 2024; Apple Mail, Yahoo,
			// and most enterprise clients render a native "Unsubscribe"
			// button in the inbox UI when both are present.
			Headers: map[string]string{
				"List-Unsubscribe":      "<" + unsubURL + ">",
				"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			},
		}
		if _, err := c.client.Emails.Send(params); err != nil {
			result.Failed++
			failures = append(failures, fmt.Sprintf("%s: %v", addr, err))
			// On 429 specifically, give the bucket extra refill time.
			// resend-go wraps the body verbatim; substring match is
			// best-effort but covers the dominant case.
			if strings.Contains(err.Error(), "429") || strings.Contains(strings.ToLower(err.Error()), "rate") {
				time.Sleep(2 * time.Second)
			}
			continue
		}
		result.Succeeded++
	}
	if len(failures) > 0 {
		result.failureDetail = strings.Join(failures, "; ")
	}
	return result
}
