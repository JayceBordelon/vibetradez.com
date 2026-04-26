package rollouts

import "vibetradez.com/internal/templates"

/*
autoExecutionLiveV1 announces the auto-execution feature shipped in
PR #43. First rollout email defined for this service.

Slug is permanent — never rename. Future redeploys check this slug
in the sent_rollouts table and skip-if-present so the email goes
out exactly once across the lifetime of the deployment.
*/
var autoExecutionLiveV1 = Rollout{
	Slug:    "auto-execution-live-v1",
	Subject: "Big update: VibeTradez can now execute trades — here's how to follow along",
	Render:  templates.RenderRolloutAutoExecutionLive,
}
