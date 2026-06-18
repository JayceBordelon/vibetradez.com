/*
Shared API types. The portfolio surface has its own types in
types/portfolio.ts.
*/

export interface ApiResponse {
  ok: boolean;
  message: string;
}

/*
TranscriptEvent mirrors the Go transcript.Event: one entry in the ordered
conversation stream captured during a daily run. `round` groups a turn's
narration, tool calls, and their results.
*/
export interface TranscriptEvent {
  round: number;
  type: "text" | "thinking" | "tool_use" | "tool_result" | "session_marker";
  text?: string;
  tool_name?: string;
  tool_use_id?: string;
  tool_input?: unknown;
  tool_result?: string;
  // Set only on session_marker events: that session's token usage + wall-clock,
  // for the per-session cost breakdown.
  usage?: TranscriptUsage;
  duration_ms?: number;
}

/*
TranscriptUsage mirrors the Go transcript.Usage: the session token spend
summed across every API round, plus the round count. InputTokens is the
fresh (uncached) input billed; cache_read_tokens is what the prompt cache
served instead.
*/
export interface TranscriptUsage {
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens: number;
  web_search_requests: number;
  web_fetch_requests: number;
  rounds: number;
}

/*
TranscriptResponse is GET /api/transcript?date=&kind=. `available` is false
(with an empty events array) when no transcript was captured for that
date+kind. kind is "portfolio" for the daily portfolio session. `usage` and
`duration_ms` summarize the session's token spend and wall-clock time.
*/
export interface TranscriptResponse {
  date: string;
  kind: "selection" | "execution" | "portfolio";
  model: string;
  available: boolean;
  created_at?: string;
  events: TranscriptEvent[];
  usage?: TranscriptUsage;
  duration_ms?: number;
}
