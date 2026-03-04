package cmds

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

func emitSeedTurnToProgram(p *tea.Program, t *turns.Turn) {
	if p == nil || t == nil || len(t.Blocks) == 0 {
		return
	}
	for _, b := range t.Blocks {
		var role string
		switch b.Kind {
		case turns.BlockKindUser:
			role = "user"
		case turns.BlockKindLLMText:
			role = "assistant"
		case turns.BlockKindToolCall,
			turns.BlockKindToolUse,
			turns.BlockKindSystem,
			turns.BlockKindReasoning,
			turns.BlockKindOther:
			continue
		}
		text, ok := b.Payload[turns.PayloadKeyText].(string)
		if !ok || text == "" {
			continue
		}
		p.Send(timeline.UIEntityCreated{
			ID:        timeline.EntityID{LocalID: b.ID, Kind: "llm_text"},
			Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
			Props:     map[string]any{"role": role, "text": text, "streaming": false},
			StartedAt: time.Now(),
		})
		p.Send(timeline.UIEntityCompleted{
			ID:     timeline.EntityID{LocalID: b.ID, Kind: "llm_text"},
			Result: map[string]any{"text": text},
		})
	}
}
