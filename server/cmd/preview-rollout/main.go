package main

import (
	"fmt"
	"os"
	"vibetradez.com/internal/templates"
)

func main() {
	html, err := templates.RenderRolloutAutoExecutionLive()
	if err != nil {
		fmt.Fprintln(os.Stderr, "render error:", err)
		os.Exit(1)
	}
	fmt.Print(html)
}
