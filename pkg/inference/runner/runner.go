package runner

import (
	"context"
	"errors"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

// UpdateOptions controls how conversation state is updated after inference.
type UpdateOptions struct {
	FilterBlocks func([]turns.Block) []turns.Block
}

// RunOptions configures a shared inference run.
type RunOptions struct {
	ToolRegistry geptools.ToolRegistry
	ToolConfig   *toolhelpers.ToolConfig
	SnapshotHook toolhelpers.SnapshotHook
	EventSinks   []events.EventSink
	Update       UpdateOptions
}

// Run executes inference using a conversation state snapshot and updates the state.
// If ToolRegistry is nil, it runs a single inference pass without tools.
func Run(ctx context.Context, eng engine.Engine, state **conversation.ConversationState, prompt string, opts RunOptions) (*turns.Turn, error) {
	if eng == nil {
		return nil, errors.New("engine is nil")
	}
	if state != nil && *state == nil {
		*state = conversation.NewConversationState("")
	}

	var seed *turns.Turn
	var err error
	if state != nil {
		seed, err = SnapshotForPrompt(*state, prompt)
	} else {
		seed, err = SnapshotForPrompt(nil, prompt)
	}
	if err != nil {
		return nil, err
	}

	runCtx := ctx
	if len(opts.EventSinks) > 0 {
		runCtx = events.WithEventSinks(runCtx, opts.EventSinks...)
	}
	if opts.SnapshotHook != nil {
		runCtx = toolhelpers.WithTurnSnapshotHook(runCtx, opts.SnapshotHook)
	}

	var updated *turns.Turn
	if opts.ToolRegistry != nil {
		cfg := toolhelpers.NewToolConfig()
		if opts.ToolConfig != nil {
			cfg = *opts.ToolConfig
		}
		updated, err = toolhelpers.RunToolCallingLoop(runCtx, eng, seed, opts.ToolRegistry, cfg)
	} else {
		updated, err = eng.RunInference(runCtx, seed)
	}
	if updated != nil && state != nil {
		UpdateStateFromTurn(state, updated, opts.Update)
	}
	return updated, err
}

// SnapshotForPrompt builds a Turn snapshot from the state and appends the user prompt.
func SnapshotForPrompt(state *conversation.ConversationState, prompt string) (*turns.Turn, error) {
	temp := conversation.NewConversationState("")
	if state != nil {
		temp.ID = state.ID
		temp.RunID = state.RunID
		temp.Blocks = append([]turns.Block(nil), state.Blocks...)
		temp.Data = state.Data.Clone()
		temp.Metadata = state.Metadata.Clone()
		temp.Version = state.Version
	}
	if prompt != "" {
		if err := temp.Apply(conversation.MutateAppendUserText(prompt)); err != nil {
			return nil, err
		}
	}
	cfg := conversation.DefaultSnapshotConfig()
	return temp.Snapshot(cfg)
}

// UpdateStateFromTurn persists the latest turn into the conversation state.
func UpdateStateFromTurn(state **conversation.ConversationState, t *turns.Turn, opts UpdateOptions) {
	if t == nil || state == nil {
		return
	}
	if *state == nil {
		*state = conversation.NewConversationState(t.RunID)
	}
	blocks := t.Blocks
	if opts.FilterBlocks != nil {
		blocks = opts.FilterBlocks(blocks)
	}
	(*state).Blocks = append([]turns.Block(nil), blocks...)
	(*state).Data = t.Data.Clone()
	(*state).Metadata = t.Metadata.Clone()
	if t.RunID != "" {
		(*state).RunID = t.RunID
	}
}
