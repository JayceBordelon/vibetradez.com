package rollouts

import "vibetradez.com/internal/templates"

/*
transcriptsV11 announces the daily model-reasoning transcripts. Every
trading day the picker (9:25 ET) and the at-open agent (9:30 ET) run
tool-use conversations; their narration, tool calls, and decisions are
now captured and published on the dashboard at /transcript/<date>/<kind>
(kind = selection or execution), reachable from the dashboard, each
trade page, and the history view. Account balances are redacted before
storage.

Slug is permanent. The retired v10 slug ("opus-4-8-upgrade-v10") stays
recorded in sent_rollouts forever; this v11 slug is the new permission
slip.
*/
var transcriptsV11 = Rollout{
	Slug:    "daily-transcripts-v11",
	Subject: "You can now read the reasoning behind every trade",
	Render:  templates.RenderRolloutTranscripts,
}
