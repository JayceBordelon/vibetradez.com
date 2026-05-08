/*
Standalone helper to preview a rollout email locally. Pass the slug
as the first arg, defaults to the most recent rollout. Run with:

	go run ./cmd/preview-rollout > /tmp/rollout.html && open /tmp/rollout.html
	go run ./cmd/preview-rollout auto-execution-live-v1 > /tmp/rollout.html && open /tmp/rollout.html
*/
package main

import (
	"fmt"
	"os"

	"vibetradez.com/internal/templates"
)

func main() {
	slug := "equity-walk-day-one-v5"
	if len(os.Args) > 1 {
		slug = os.Args[1]
	}

	var (
		html string
		err  error
	)
	switch slug {
	case "live-trading-starts-v3":
		html, err = templates.RenderRolloutLiveTradingStarts()
	case "claude-only-v2":
		html, err = templates.RenderRolloutClaudeOnly()
	case "auto-execution-live-v1":
		html, err = templates.RenderRolloutAutoExecutionLive()
	case "basket-and-copy-trading-v4":
		html, err = templates.RenderRolloutBasketAndCopyTrading()
	case "equity-walk-day-one-v5":
		html, err = templates.RenderRolloutEquityWalkDayOne()
	default:
		fmt.Fprintf(os.Stderr, "unknown rollout slug: %s\n", slug)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "render error:", err)
		os.Exit(1)
	}
	fmt.Print(html)
}
