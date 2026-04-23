package observability

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventJSONRoundTrip(t *testing.T) {
	ev := Event{
		Time:     time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		RunID:    "run-1",
		Worktree: "feat/x",
		Step:     "implement",
		Kind:     KindStepStarted,
		Payload:  map[string]any{"iteration": float64(1)},
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !got.Time.Equal(ev.Time) {
		t.Errorf("time mismatch: got %v want %v", got.Time, ev.Time)
	}
	if got.RunID != ev.RunID || got.Worktree != ev.Worktree || got.Step != ev.Step {
		t.Errorf("string fields mismatch: %+v", got)
	}
	if got.Kind != ev.Kind {
		t.Errorf("kind mismatch: got %q want %q", got.Kind, ev.Kind)
	}
	if got.Payload["iteration"] != float64(1) {
		t.Errorf("payload mismatch: %+v", got.Payload)
	}
}

func TestEventOmitsEmptyFields(t *testing.T) {
	ev := Event{
		Time: time.Now(),
		Kind: KindRunStarted,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(data)
	for _, missing := range []string{`"worktree"`, `"step"`, `"payload"`, `"run_id"`} {
		if containsString(s, missing) {
			t.Errorf("expected %s to be omitted, got %s", missing, s)
		}
	}
}

func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
