package rollouts

import "vibetradez.com/internal/templates"

/*
claudeOnlyV2 announces the removal of the dual-model picker in favour
of a single Claude-only pipeline. Subscribers get one heads-up email
explaining what changed and why; subsequent deploys are no-ops because
the slug is recorded in sent_rollouts on first send.

Slug is permanent, never rename.
*/
var claudeOnlyV2 = Rollout{
	Slug:    "claude-only-v2",
	Subject: "We benched ChatGPT. Claude takes the floor.",
	Render:  templates.RenderRolloutClaudeOnly,
}
