package rollouts

import "vibetradez.com/internal/templates"

/*
opus48UpgradeV10 announces that the trade picker (9:25 ET) and the
at-open agent (9:30 ET) have both moved to Claude Opus 4.8, Anthropic's
newest and most capable production model, from Opus 4.7. The two-stage
flow, the three-candidate basket, and every hard cap at the tool layer
are unchanged; only the underlying model changed.

Slug is permanent. The retired v9 slug ("multi-agent-live-v9") stays
recorded in sent_rollouts forever; this v10 slug is the new permission
slip.
*/
var opus48UpgradeV10 = Rollout{
	Slug:    "opus-4-8-upgrade-v10",
	Subject: "Your trade picker just upgraded to Claude Opus 4.8",
	Render:  templates.RenderRolloutOpus48,
}
