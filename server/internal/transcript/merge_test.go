package transcript

import "testing"

// Merge must shift the later session's rounds past the prior session's
// highest round, drop a labeled separator at the seam, and sum usage +
// duration, so the three intraday sessions read as one continuous daily log.
func TestMergeOffsetsRoundsAndSumsUsage(t *testing.T) {
	prior := Transcript{
		Model: "m",
		Events: []Event{
			{Round: 0, Type: EventText, Text: "open-a"},
			{Round: 1, Type: EventText, Text: "open-b"},
		},
		Usage:      Usage{InputTokens: 10, OutputTokens: 5, Rounds: 2},
		DurationMS: 100,
	}
	next := Transcript{
		Model: "m",
		Events: []Event{
			{Round: 0, Type: EventText, Text: "mid-a"},
			{Round: 1, Type: EventText, Text: "mid-b"},
		},
		Usage:      Usage{InputTokens: 7, OutputTokens: 3, Rounds: 2},
		DurationMS: 50,
	}

	got := Merge(prior, next, "SEP")

	if len(got.Events) != 5 { // 2 prior + separator + 2 next
		t.Fatalf("event count: got %d want 5", len(got.Events))
	}
	if sep := got.Events[2]; sep.Type != EventText || sep.Text != "SEP" || sep.Round != 2 {
		t.Fatalf("separator at seam: got %+v", sep)
	}
	if got.Events[3].Round != 2 || got.Events[4].Round != 3 {
		t.Fatalf("next rounds offset: got %d,%d want 2,3", got.Events[3].Round, got.Events[4].Round)
	}
	if got.Events[3].Text != "mid-a" {
		t.Fatalf("next order preserved: got %q want mid-a", got.Events[3].Text)
	}
	if got.Usage.InputTokens != 17 || got.Usage.OutputTokens != 8 || got.Usage.Rounds != 4 {
		t.Fatalf("usage summed: got %+v", got.Usage)
	}
	if got.DurationMS != 150 {
		t.Fatalf("duration summed: got %d want 150", got.DurationMS)
	}
}

// The first session of the day merges into a zero-value prior, which simply
// prepends the separator (offset 0) so even a standalone session is labeled.
func TestMergeIntoEmptyPriorLabelsFirstSession(t *testing.T) {
	next := Transcript{
		Events:     []Event{{Round: 0, Type: EventText, Text: "a"}},
		Usage:      Usage{Rounds: 1},
		DurationMS: 10,
	}

	got := Merge(Transcript{}, next, "OPEN")

	if len(got.Events) != 2 {
		t.Fatalf("event count: got %d want 2", len(got.Events))
	}
	if got.Events[0].Text != "OPEN" || got.Events[0].Round != 0 {
		t.Fatalf("separator first: got %+v", got.Events[0])
	}
	if got.Events[1].Round != 0 || got.Events[1].Text != "a" {
		t.Fatalf("next event: got %+v", got.Events[1])
	}
}
