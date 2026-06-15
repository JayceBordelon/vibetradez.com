package portfolio

import _ "embed"

/*
SystemPrompt is the daily portfolio agent's instructions. It is authored in
prompt.md (right next to this file) and embedded at build time, so the prompt
can be viewed and edited as a standalone Markdown document without touching
Go. buildPrompt fills its two %s verbs, in order:

  1. today's date (YYYY-MM-DD)
  2. weekday

The live book and the prior session are NOT interpolated: the model reads
them itself through get_portfolio / get_recent_decisions / get_track_record.

Authoring notes (read before editing prompt.md):

  - The tool guards in tools.go and guards.go enforce the settled-cash rule
    and sell validation regardless of what the prompt says. Repeating them in
    the prompt is reinforcement, not contract.
  - Keep exactly two %s verbs, in the order above. Add nothing else that
    looks like a format verb (a literal percent must be written %%).
  - The final-response JSON shape is parsed strictly. Do not change it without
    updating agent.go's parser.
*/
//go:embed prompt.md
var SystemPrompt string
