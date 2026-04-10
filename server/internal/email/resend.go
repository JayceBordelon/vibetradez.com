package email

import (
	"fmt"

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
