package webchat

import (
	"testing"

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
