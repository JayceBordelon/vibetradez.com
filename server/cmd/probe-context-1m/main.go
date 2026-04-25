// Standalone probe to verify the Anthropic 1M-context beta header is
// accepted by the configured Claude model. Run locally with a real
// ANTHROPIC_API_KEY before wiring the header into the trading server.
//
//	cd vibetradez.com/server
//	ANTHROPIC_API_KEY=sk-ant-... ANTHROPIC_MODEL=claude-opus-4-7 \
//	  go run ./cmd/probe-context-1m
//
// Exits 0 on success and prints OK plus the model + a snippet of the
// reply. Exits 1 on any error and prints the full error so we can tell
// "header rejected" from "key invalid" from "model gone."
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"vibetradez.com/internal/config"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "ANTHROPIC_API_KEY is required")
		os.Exit(1)
	}

	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = config.DefaultAnthropicModel
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(60*time.Second),
		option.WithHeader("anthropic-beta", "context-1m-2025-08-07"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 32,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("ping")),
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}

	var reply string
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			reply += tb.Text
		}
	}

	fmt.Printf("OK model=%s reply=%q usage_in=%d usage_out=%d\n",
		model, reply, msg.Usage.InputTokens, msg.Usage.OutputTokens)
}
