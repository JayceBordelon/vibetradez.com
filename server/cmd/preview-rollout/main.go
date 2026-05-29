/*
Standalone helper to preview a rollout email locally. Pass the slug
as the first arg, defaults to the most recent pending rollout. Run:

	go run ./cmd/preview-rollout > /tmp/rollout.html && open /tmp/rollout.html
	go run ./cmd/preview-rollout opus-4-8-upgrade-v10 > /tmp/rollout.html && open /tmp/rollout.html
*/
package main

import (
	"fmt"
	"os"

	"vibetradez.com/internal/templates"
)

func main() {
	slug := "opus-4-8-upgrade-v10"
	if len(os.Args) > 1 {
		slug = os.Args[1]
	}

	var (
		html string
		err  error
	)
	switch slug {
	case "opus-4-8-upgrade-v10":
		html, err = templates.RenderRolloutOpus48()
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
