package webchat

import (
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

func TestBuildSeedTurn_AppendsSystemPromptBlock(t *testing.T) {
	seed := buildSeedTurn("run-1", "You are an assistant")
	if seed == nil {
		t.Fatal("expected seed turn")
	}
	if len(seed.Blocks) == 0 {
		t.Fatal("expected seed turn to include system prompt block")
	}
}

func TestAppendNewTurnFromUserPrompt_EmptyPromptKeepsSystemBlock(t *testing.T) {
	seed := buildSeedTurn("run-2", "You are an assistant")
	sess := &session.Session{SessionID: "run-2", Turns: []*turns.Turn{seed}}
	turn, err := sess.AppendNewTurnFromUserPrompt("")
	if err != nil {
		t.Fatalf("AppendNewTurnFromUserPrompt error: %v", err)
	}
	if turn == nil || len(turn.Blocks) == 0 {
		t.Fatal("expected appended turn to retain system prompt block")
	}
}

func TestBuildConversationRecord_IncludesLastSeenVersion(t *testing.T) {
	now := time.UnixMilli(1739960001000)
	conv := &Conversation{
		ID:              "conv-1",
		SessionID:       "sess-1",
		RuntimeKey:      "default",
		createdAt:       now.Add(-time.Minute),
		lastActivity:    now,
		timelineProj:    &TimelineProjector{},
		lastSeenVersion: 42,
	}

	rec := buildConversationRecord(conv, "active", "")
	if rec.ConvID != "conv-1" {
		t.Fatalf("unexpected conv id: %q", rec.ConvID)
	}
	if rec.LastSeenVersion != 42 {
		t.Fatalf("expected last seen version 42, got %d", rec.LastSeenVersion)
	}
	if !rec.HasTimeline {
		t.Fatal("expected has_timeline to be true when projector is present")
	}
}
