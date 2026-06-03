/*
Standalone helper to preview a rollout email locally. Pass the slug
as the first arg, defaults to the most recent pending rollout. Run:

	go run ./cmd/preview-rollout > /tmp/rollout.html && open /tmp/rollout.html
	go run ./cmd/preview-rollout daily-transcripts-v11 > /tmp/rollout.html && open /tmp/rollout.html
*/
package main

import (
	"fmt"
	"os"

	"vibetradez.com/internal/templates"
)

func main() {
	slug := "daily-transcripts-v11"
	if len(os.Args) > 1 {
		slug = os.Args[1]
	}

	var (
		html string
		err  error
	)
	switch slug {
	case "daily-transcripts-v11":
		html, err = templates.RenderRolloutTranscripts()
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
