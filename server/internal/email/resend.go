package email

import (
	"fmt"
	"strings"

	"github.com/resend/resend-go/v2"
)

type Client struct {
	client *resend.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		client: resend.NewClient(apiKey),
	}
}

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
func (c *Client) SendPersonalizedToList(from string, to []string, subject, htmlContent string, unsubURLFor func(email string) string) error {
	const placeholder = "@@VT_UNSUBSCRIBE_URL@@"
	var failures []string
	for _, addr := range to {
		body := strings.ReplaceAll(htmlContent, placeholder, unsubURLFor(addr))
		params := &resend.SendEmailRequest{
			From:    from,
			To:      []string{addr},
			Subject: subject,
			Html:    body,
		}
		if _, err := c.client.Emails.Send(params); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", addr, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("personalized send had %d failure(s) of %d recipient(s): %s", len(failures), len(to), strings.Join(failures, "; "))
	}
	return nil
}
