package portfolio

import _ "embed"

/*
SystemPrompt is the daily portfolio agent's instructions. It is authored in
prompt.md (right next to this file) and embedded at build time, so the prompt
can be viewed and edited as a standalone Markdown document without touching
Go. buildPrompt fills its five %s verbs, in order:

  1. today's date (YYYY-MM-DD)
  2. weekday
  3. the live portfolio snapshot JSON
  4. WHERE YOU LEFT OFF (the prior session's synopsis + action items)
  5. the hard-caps summary (rendered from caps.go so prompt + code never drift)

Authoring notes (read before editing prompt.md):

  - The tool guards in tools.go and caps.go enforce every hard cap regardless
    of what the prompt says. Repeating the caps is reinforcement, not contract.
  - Keep exactly five %s verbs, in the order above. Add nothing else that
    looks like a format verb (a literal percent must be written %%).
  - The final-response JSON shape is parsed strictly. Do not change it without
    updating agent.go's parser.
*/
//go:embed prompt.md
var SystemPrompt string
