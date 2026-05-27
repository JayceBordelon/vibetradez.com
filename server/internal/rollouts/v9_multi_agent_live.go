package rollouts

import "vibetradez.com/internal/templates"

/*
multiAgentLiveV9 announces that the multi-agent picker is fully
visible on the dashboard now that today's two-part fix has shipped.
The picker + at-open agent flow has been firing real trades for a
handful of sessions, but two dashboard bugs were hiding the most
interesting half of it: skipped candidates rendered an endless
"finding contracts..." placeholder instead of the agent's written
reason, and the live option mark / bid / ask / volume skeleton
pulsed forever when the at-open agent resolved its contract after
the quotes hub had already computed its subscription set. Both
fixes landed in the same deploy this rollout chases.

Slug is permanent. The retired v8 slug ("agent-executes-v8") stays
recorded in sent_rollouts forever; this v9 slug is the new
permission slip.
*/
var multiAgentLiveV9 = Rollout{
	Slug:    "multi-agent-live-v9",
	Subject: "The multi-agent picker is live on your dashboard",
	Render:  templates.RenderRolloutMultiAgentLive,
}
