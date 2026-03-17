package webchat

import (
	"errors"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/turns"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

type llmConversationState struct {
	runtimeFingerprint string
	engine             engine.Engine
	session            *session.Session
	seedSystemPrompt   string
	toolNames          []string
}

func toolNamesFromResolvedRuntime(runtime *infruntime.ConversationRuntimeRequest) []string {
	if runtime == nil || runtime.ResolvedProfileRuntime == nil {
		return nil
	}
	tools := runtime.ResolvedProfileRuntime.Tools
	if len(tools) == 0 {
		return nil
	}
	ret := make([]string, 0, len(tools))
	for _, name := range tools {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		ret = append(ret, name)
	}
	if len(ret) == 0 {
		return nil
	}
	return ret
}

func (cm *ConvManager) ensureLLMState(conv *Conversation) (*llmConversationState, error) {
	if cm == nil {
		return nil, errors.New("conversation manager is nil")
	}
	if conv == nil {
		return nil, errors.New("conversation is nil")
	}
	if cm.runtimeComposer == nil {
		return nil, errors.New("conversation manager missing runtime composer")
	}

	conv.mu.Lock()
	if conv.llm != nil && conv.llm.runtimeFingerprint == conv.RuntimeFingerprint {
		state := conv.llm
		conv.mu.Unlock()
		return state, nil
	}
	req := infruntime.ConversationRuntimeRequest{
		ConvID:                     conv.ID,
		ProfileKey:                 conv.RuntimeKey,
		ProfileVersion:             conv.profileVersion,
		ResolvedStepSettings:       cloneStepSettings(conv.resolvedStepSettings),
		ResolvedProfileRuntime:     conv.resolvedRuntime,
		ResolvedProfileFingerprint: strings.TrimSpace(conv.RuntimeFingerprint),
	}
	sessionID := conv.SessionID
	sink := conv.Sink
	runtimeFingerprint := strings.TrimSpace(conv.RuntimeFingerprint)
	conv.mu.Unlock()

	runtime, err := cm.runtimeComposer.Compose(cm.baseCtx, req)
	if err != nil {
		return nil, err
	}
	if runtime.Engine == nil {
		return nil, errors.New("runtime composer returned nil engine")
	}
	if sink == nil {
		return nil, errors.New("conversation sink is nil")
	}
	if strings.TrimSpace(runtime.RuntimeFingerprint) == "" {
		runtime.RuntimeFingerprint = runtimeFingerprint
	}

	state := &llmConversationState{
		runtimeFingerprint: strings.TrimSpace(runtime.RuntimeFingerprint),
		engine:             runtime.Engine,
		seedSystemPrompt:   runtime.SeedSystemPrompt,
		toolNames:          toolNamesFromResolvedRuntime(&req),
	}
	state.session = &session.Session{
		SessionID: sessionID,
		Turns:     []*turns.Turn{buildSeedTurn(sessionID, runtime.SeedSystemPrompt)},
	}

	conv.mu.Lock()
	defer conv.mu.Unlock()
	if conv.llm != nil && conv.llm.runtimeFingerprint == conv.RuntimeFingerprint {
		return conv.llm, nil
	}
	conv.llm = state
	return state, nil
}

func buildSeedTurn(sessionID string, systemPrompt string) *turns.Turn {
	seed := &turns.Turn{}
	if strings.TrimSpace(systemPrompt) != "" {
		turns.AppendBlock(seed, turns.NewSystemTextBlock(systemPrompt))
	}
	_ = turns.KeyTurnMetaSessionID.Set(&seed.Metadata, sessionID)
	return seed
}
