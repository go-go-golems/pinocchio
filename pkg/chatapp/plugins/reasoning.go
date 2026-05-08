package plugins

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

const (
	// ReasoningStartedEventName is the backend event published when a thinking stream begins.
	ReasoningStartedEventName = "ChatReasoningStarted"
	// ReasoningDeltaEventName is the backend event published for each thinking token delta.
	ReasoningDeltaEventName = "ChatReasoningDelta"
	// ReasoningFinishedEventName is the backend event published when thinking ends or a reasoning summary arrives.
	ReasoningFinishedEventName = "ChatReasoningFinished"

	// ReasoningStartedUIName is the UI event emitted when a thinking stream begins.
	ReasoningStartedUIName = "ChatReasoningStarted"
	// ReasoningAppendedUIName is the UI event emitted for each thinking token delta.
	ReasoningAppendedUIName = "ChatReasoningAppended"
	// ReasoningFinishedUIName is the UI event emitted when thinking ends.
	ReasoningFinishedUIName = "ChatReasoningFinished"
)

// ReasoningPlugin is a ChatPlugin that handles thinking/reasoning streams from
// geppetto inference engines. It translates EventThinkingPartial and EventInfo
// (thinking-started, thinking-ended, reasoning-summary) into sessionstream
// events, and projects them into ChatMessage timeline entities with role "thinking".
//
// Each contiguous thinking phase gets its own segment ID derived from the
// parent assistant message ID (for example, "chat-msg-5:thinking:1",
// "chat-msg-5:thinking:2").
type ReasoningPlugin struct {
	mu             sync.Mutex
	segments       map[reasoningKey]reasoningSegmentState
	activeByParent map[string]reasoningKey
	nextSegment    map[string]int32
}

type reasoningKey struct {
	ParentMessageID string
	Provider        string
	ResponseID      string
	ItemID          string
	OutputIndex     int32
	HasOutputIndex  bool
	ChoiceIndex     int32
	HasChoiceIndex  bool
	StreamKind      string
	CorrelationKey  string
}

type reasoningSegmentState struct {
	Current  int32
	Active   bool
	Provider reasoningProviderInfo
}

type reasoningProviderInfo struct {
	Provider       string
	ResponseID     string
	ItemID         string
	OutputIndex    *int32
	SummaryIndex   *int32
	ChoiceIndex    *int32
	StreamKind     string
	CorrelationKey string
	ToolCallID     string
	ToolCallIndex  *int32
}

// NewReasoningPlugin creates a new ReasoningPlugin.
func NewReasoningPlugin() chatapp.ChatPlugin {
	return &ReasoningPlugin{
		segments:       map[reasoningKey]reasoningSegmentState{},
		activeByParent: map[string]reasoningKey{},
		nextSegment:    map[string]int32{},
	}
}

// RegisterSchemas registers the reasoning event and UI event payload schemas.
func (p *ReasoningPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(ReasoningStartedEventName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterEvent(ReasoningDeltaEventName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterEvent(ReasoningFinishedEventName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterUIEvent(ReasoningStartedUIName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterUIEvent(ReasoningAppendedUIName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterUIEvent(ReasoningFinishedUIName, &chatappv1.ReasoningUpdate{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// HandleRuntimeEvent handles EventThinkingPartial and EventInfo events.
func (p *ReasoningPlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	parentMessageID := strings.TrimSpace(runtime.MessageID)
	if parentMessageID == "" {
		return false, nil
	}

	switch ev := event.(type) {
	case *gepevents.EventThinkingPartial:
		providerInfo := reasoningProviderInfoFromMetadata(ev.Metadata())
		reasoningMessageID, segment, providerInfo := p.ensureReasoningSegment(parentMessageID, providerInfo)
		return true, runtime.Publish(ctx, ReasoningDeltaEventName, applyReasoningProviderInfo(&chatappv1.ReasoningUpdate{
			MessageId:       reasoningMessageID,
			ParentMessageId: parentMessageID,
			Segment:         segment,
			Role:            "thinking",
			Chunk:           ev.Delta,
			Content:         ev.Completion,
			Text:            ev.Completion,
			Status:          "streaming",
			Streaming:       true,
			Source:          "thinking",
			SegmentType:     "thinking",
		}, providerInfo))
	case *gepevents.EventInfo:
		switch ev.Message {
		case "thinking-started":
			providerInfo := reasoningProviderInfoFromData(ev.Data)
			reasoningMessageID, segment, providerInfo := p.startReasoningSegment(parentMessageID, providerInfo)
			return true, runtime.Publish(ctx, ReasoningStartedEventName, applyReasoningProviderInfo(&chatappv1.ReasoningUpdate{
				MessageId:       reasoningMessageID,
				ParentMessageId: parentMessageID,
				Segment:         segment,
				Role:            "thinking",
				Status:          "streaming",
				Streaming:       true,
				Source:          "thinking",
				SegmentType:     "thinking",
			}, providerInfo))
		case "thinking-ended":
			providerInfo := reasoningProviderInfoFromData(ev.Data)
			reasoningMessageID, segment, providerInfo, key, ok := p.currentReasoningSegment(parentMessageID, providerInfo)
			if !ok {
				return false, nil
			}
			p.finishReasoningSegment(key)
			return true, runtime.Publish(ctx, ReasoningFinishedEventName, applyReasoningProviderInfo(&chatappv1.ReasoningUpdate{
				MessageId:       reasoningMessageID,
				ParentMessageId: parentMessageID,
				Segment:         segment,
				Role:            "thinking",
				Status:          "finished",
				Streaming:       false,
				Source:          "thinking",
				SegmentType:     "thinking",
			}, providerInfo))
		case "reasoning-summary-started", "reasoning-summary-ended":
			p.updateReasoningProviderInfo(parentMessageID, reasoningProviderInfoFromData(ev.Data))
			return false, nil
		case "reasoning-summary":
			providerInfo := reasoningProviderInfoFromData(ev.Data)
			reasoningMessageID, segment, providerInfo, key := p.summaryReasoningSegment(parentMessageID, providerInfo)
			p.finishReasoningSegment(key)
			text := infoText(ev.Data)
			return true, runtime.Publish(ctx, ReasoningFinishedEventName, applyReasoningProviderInfo(&chatappv1.ReasoningUpdate{
				MessageId:       reasoningMessageID,
				ParentMessageId: parentMessageID,
				Segment:         segment,
				Role:            "thinking",
				Content:         text,
				Text:            text,
				Status:          "finished",
				Streaming:       false,
				Source:          "summary",
				SegmentType:     "thinking",
			}, providerInfo))
		default:
			return false, nil
		}
	default:
		return false, nil
	}
}

// ProjectUI projects reasoning backend events into UI events.
func (p *ReasoningPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	payload, ok := reasoningProjectedPayload(ev, view)
	if !ok {
		return nil, false, nil
	}
	switch ev.Name {
	case ReasoningStartedEventName:
		return []sessionstream.UIEvent{{Name: ReasoningStartedUIName, Payload: payload}}, true, nil
	case ReasoningDeltaEventName:
		return []sessionstream.UIEvent{{Name: ReasoningAppendedUIName, Payload: payload}}, true, nil
	case ReasoningFinishedEventName:
		return []sessionstream.UIEvent{{Name: ReasoningFinishedUIName, Payload: payload}}, true, nil
	default:
		return nil, false, nil
	}
}

// ProjectTimeline projects reasoning backend events into ChatMessage timeline entities.
// Thinking entities use role "thinking" and accumulate content across deltas.
func (p *ReasoningPlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	payload, ok := reasoningProjectedPayload(ev, view)
	if !ok {
		return nil, false, nil
	}
	messageID := payload.GetMessageId()
	if messageID == "" {
		return nil, true, nil
	}
	entity, hadEntity := currentReasoningEntity(view, messageID)
	content := payload.GetContent()
	if content == "" {
		content = entity.GetContent()
		if content == "" {
			content = entity.GetText()
		}
	}
	if content == "" && !hadEntity {
		return nil, true, nil
	}

	entity.MessageId = messageID
	entity.Role = "thinking"
	entity.Content = content
	entity.Text = content
	entity.ParentMessageId = payload.GetParentMessageId()
	entity.Segment = payload.GetSegment()
	entity.SegmentType = "thinking"

	switch ev.Name {
	case ReasoningStartedEventName, ReasoningDeltaEventName:
		entity.Status = "streaming"
		entity.Streaming = true
	case ReasoningFinishedEventName:
		entity.Status = "finished"
		entity.Streaming = false
	default:
		return nil, false, nil
	}

	return []sessionstream.TimelineEntity{{Kind: chatapp.TimelineEntityChatMessage, Id: messageID, Payload: entity}}, true, nil
}

// ReasoningEntityID returns the first thinking segment ID for a given parent message ID.
func ReasoningEntityID(messageID string) string {
	return ReasoningSegmentEntityID(messageID, 1)
}

// ReasoningSegmentEntityID returns the thinking message ID for a specific parent
// assistant message and reasoning segment number.
func ReasoningSegmentEntityID(messageID string, segment int32) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" || segment <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:thinking:%d", messageID, segment)
}

func (p *ReasoningPlugin) startReasoningSegment(parentMessageID string, providerInfo reasoningProviderInfo) (string, int32, reasoningProviderInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureStateLocked()

	key := reasoningKeyFromProviderInfo(parentMessageID, providerInfo)
	state := p.segments[key]
	if !state.Active {
		state.Current = p.nextSegmentLocked(parentMessageID)
		state.Active = true
	}
	state.Provider = state.Provider.merge(providerInfo)
	p.segments[key] = state
	p.activeByParent[parentMessageID] = key
	return ReasoningSegmentEntityID(parentMessageID, state.Current), state.Current, state.Provider
}

func (p *ReasoningPlugin) ensureReasoningSegment(parentMessageID string, providerInfo reasoningProviderInfo) (string, int32, reasoningProviderInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureStateLocked()

	key := p.resolveReasoningKeyLocked(parentMessageID, providerInfo)
	state := p.segments[key]
	if !state.Active {
		state.Current = p.nextSegmentLocked(parentMessageID)
		state.Active = true
	}
	state.Provider = state.Provider.merge(providerInfo)
	p.segments[key] = state
	p.activeByParent[parentMessageID] = key
	return ReasoningSegmentEntityID(parentMessageID, state.Current), state.Current, state.Provider
}

func (p *ReasoningPlugin) currentReasoningSegment(parentMessageID string, providerInfo reasoningProviderInfo) (string, int32, reasoningProviderInfo, reasoningKey, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureStateLocked()

	key := p.resolveReasoningKeyLocked(parentMessageID, providerInfo)
	state := p.segments[key]
	if !state.Active || state.Current <= 0 {
		return "", 0, reasoningProviderInfo{}, reasoningKey{}, false
	}
	state.Provider = state.Provider.merge(providerInfo)
	p.segments[key] = state
	return ReasoningSegmentEntityID(parentMessageID, state.Current), state.Current, state.Provider, key, true
}

func (p *ReasoningPlugin) summaryReasoningSegment(parentMessageID string, providerInfo reasoningProviderInfo) (string, int32, reasoningProviderInfo, reasoningKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureStateLocked()

	key := p.resolveReasoningKeyLocked(parentMessageID, providerInfo)
	state := p.segments[key]
	if state.Current <= 0 {
		state.Current = p.nextSegmentLocked(parentMessageID)
	}
	state.Active = false
	state.Provider = state.Provider.merge(providerInfo)
	p.segments[key] = state
	return ReasoningSegmentEntityID(parentMessageID, state.Current), state.Current, state.Provider, key
}

func (p *ReasoningPlugin) finishReasoningSegment(key reasoningKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureStateLocked()

	state := p.segments[key]
	state.Active = false
	p.segments[key] = state
	if active, ok := p.activeByParent[key.ParentMessageID]; ok && active == key {
		delete(p.activeByParent, key.ParentMessageID)
	}
}

func (p *ReasoningPlugin) updateReasoningProviderInfo(parentMessageID string, providerInfo reasoningProviderInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureStateLocked()

	key := p.resolveReasoningKeyLocked(parentMessageID, providerInfo)
	state := p.segments[key]
	if state.Current <= 0 {
		state.Current = p.nextSegmentLocked(parentMessageID)
	}
	state.Provider = state.Provider.merge(providerInfo)
	p.segments[key] = state
}

func (p *ReasoningPlugin) ensureStateLocked() {
	if p.segments == nil {
		p.segments = map[reasoningKey]reasoningSegmentState{}
	}
	if p.activeByParent == nil {
		p.activeByParent = map[string]reasoningKey{}
	}
	if p.nextSegment == nil {
		p.nextSegment = map[string]int32{}
	}
}

func (p *ReasoningPlugin) nextSegmentLocked(parentMessageID string) int32 {
	p.nextSegment[parentMessageID]++
	return p.nextSegment[parentMessageID]
}

func (p *ReasoningPlugin) resolveReasoningKeyLocked(parentMessageID string, providerInfo reasoningProviderInfo) reasoningKey {
	key := reasoningKeyFromProviderInfo(parentMessageID, providerInfo)
	if key.HasProviderIdentity() {
		return key
	}
	if active, ok := p.activeByParent[parentMessageID]; ok {
		if state := p.segments[active]; state.Active {
			return active
		}
	}
	return key
}

func reasoningKeyFromProviderInfo(parentMessageID string, providerInfo reasoningProviderInfo) reasoningKey {
	key := reasoningKey{
		ParentMessageID: strings.TrimSpace(parentMessageID),
		Provider:        providerInfo.Provider,
		ResponseID:      providerInfo.ResponseID,
		ItemID:          providerInfo.ItemID,
	}
	if providerInfo.OutputIndex != nil {
		key.OutputIndex = *providerInfo.OutputIndex
		key.HasOutputIndex = true
	}
	if providerInfo.ChoiceIndex != nil {
		key.ChoiceIndex = *providerInfo.ChoiceIndex
		key.HasChoiceIndex = true
	}
	key.StreamKind = providerInfo.StreamKind
	key.CorrelationKey = providerInfo.CorrelationKey
	if !key.HasProviderIdentity() {
		return reasoningKey{ParentMessageID: key.ParentMessageID}
	}
	return key
}

func (key reasoningKey) HasProviderIdentity() bool {
	return key.Provider != "" || key.ResponseID != "" || key.ItemID != "" || key.HasOutputIndex || key.HasChoiceIndex || key.StreamKind != "" || key.CorrelationKey != ""
}

func (info reasoningProviderInfo) merge(next reasoningProviderInfo) reasoningProviderInfo {
	if next.Provider != "" {
		info.Provider = next.Provider
	}
	if next.ResponseID != "" {
		info.ResponseID = next.ResponseID
	}
	if next.ItemID != "" {
		info.ItemID = next.ItemID
	}
	if next.OutputIndex != nil {
		info.OutputIndex = cloneInt32Ptr(next.OutputIndex)
	}
	if next.SummaryIndex != nil {
		info.SummaryIndex = cloneInt32Ptr(next.SummaryIndex)
	}
	if next.ChoiceIndex != nil {
		info.ChoiceIndex = cloneInt32Ptr(next.ChoiceIndex)
	}
	if next.StreamKind != "" {
		info.StreamKind = next.StreamKind
	}
	if next.CorrelationKey != "" {
		info.CorrelationKey = next.CorrelationKey
	}
	if next.ToolCallID != "" {
		info.ToolCallID = next.ToolCallID
	}
	if next.ToolCallIndex != nil {
		info.ToolCallIndex = cloneInt32Ptr(next.ToolCallIndex)
	}
	return info
}

func applyReasoningProviderInfo(update *chatappv1.ReasoningUpdate, info reasoningProviderInfo) *chatappv1.ReasoningUpdate {
	if update == nil {
		return nil
	}
	update.Provider = info.Provider
	update.ResponseId = info.ResponseID
	update.ItemId = info.ItemID
	update.OutputIndex = cloneInt32Ptr(info.OutputIndex)
	update.SummaryIndex = cloneInt32Ptr(info.SummaryIndex)
	update.ChoiceIndex = cloneInt32Ptr(info.ChoiceIndex)
	update.StreamKind = info.StreamKind
	update.CorrelationKey = info.CorrelationKey
	return update
}

func reasoningProviderInfoFromMetadata(metadata gepevents.EventMetadata) reasoningProviderInfo {
	return reasoningProviderInfoFromData(metadata.Extra)
}

func reasoningProviderInfoFromData(data map[string]interface{}) reasoningProviderInfo {
	if len(data) == 0 {
		return reasoningProviderInfo{}
	}
	info := reasoningProviderInfo{}
	if v, ok := data["provider"].(string); ok {
		info.Provider = strings.TrimSpace(v)
	}
	if v, ok := data["response_id"].(string); ok {
		info.ResponseID = strings.TrimSpace(v)
	}
	if v, ok := data["item_id"].(string); ok {
		info.ItemID = strings.TrimSpace(v)
	}
	if v, ok := int32FromAny(data["output_index"]); ok {
		info.OutputIndex = &v
	}
	if v, ok := int32FromAny(data["summary_index"]); ok {
		info.SummaryIndex = &v
	}
	if v, ok := int32FromAny(data["choice_index"]); ok {
		info.ChoiceIndex = &v
	}
	if v, ok := data["stream_kind"].(string); ok {
		info.StreamKind = strings.TrimSpace(v)
	}
	if v, ok := data["correlation_key"].(string); ok {
		info.CorrelationKey = strings.TrimSpace(v)
	}
	if v, ok := data["tool_call_id"].(string); ok {
		info.ToolCallID = strings.TrimSpace(v)
	}
	if v, ok := int32FromAny(data["tool_call_index"]); ok {
		info.ToolCallIndex = &v
	}
	return info
}

func int32FromAny(v any) (int32, bool) {
	const (
		minInt32 = int64(-1 << 31)
		maxInt32 = int64(1<<31 - 1)
	)
	fromInt64 := func(n int64) (int32, bool) {
		if n < minInt32 || n > maxInt32 {
			return 0, false
		}
		return int32(n), true
	}
	fromUint64 := func(n uint64) (int32, bool) {
		if n > uint64(maxInt32) {
			return 0, false
		}
		parsed, err := strconv.ParseInt(strconv.FormatUint(n, 10), 10, 32)
		if err != nil {
			return 0, false
		}
		return int32(parsed), true
	}
	switch tv := v.(type) {
	case int:
		return fromInt64(int64(tv))
	case int32:
		return tv, true
	case int64:
		return fromInt64(tv)
	case uint:
		return fromUint64(uint64(tv))
	case uint32:
		return fromUint64(uint64(tv))
	case uint64:
		return fromUint64(tv)
	case float64:
		if tv < float64(minInt32) || tv > float64(maxInt32) || tv != float64(int32(tv)) {
			return 0, false
		}
		return int32(tv), true
	case float32:
		return int32FromAny(float64(tv))
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(tv), 10, 32)
		if err != nil {
			return 0, false
		}
		return int32(parsed), true
	default:
		return 0, false
	}
}

func cloneInt32Ptr(v *int32) *int32 {
	if v == nil {
		return nil
	}
	vv := *v
	return &vv
}

func reasoningProjectedPayload(ev sessionstream.Event, view sessionstream.TimelineView) (*chatappv1.ReasoningUpdate, bool) {
	switch ev.Name {
	case ReasoningStartedEventName, ReasoningDeltaEventName, ReasoningFinishedEventName:
		payload, ok := ev.Payload.(*chatappv1.ReasoningUpdate)
		if !ok || payload == nil {
			return nil, false
		}
		payload = proto.Clone(payload).(*chatappv1.ReasoningUpdate)
		if payload.Role == "" {
			payload.Role = "thinking"
		}
		if payload.SegmentType == "" {
			payload.SegmentType = "thinking"
		}
		if view != nil && payload.GetMessageId() != "" && payload.GetContent() == "" {
			current, _ := currentReasoningEntity(view, payload.GetMessageId())
			if currentContent := current.GetContent(); currentContent != "" {
				payload.Content = currentContent
				payload.Text = currentContent
			} else if currentText := current.GetText(); currentText != "" {
				payload.Content = currentText
				payload.Text = currentText
			}
		}
		return payload, true
	default:
		return nil, false
	}
}

func currentReasoningEntity(view sessionstream.TimelineView, id string) (*chatappv1.ChatMessageEntity, bool) {
	if view == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	entity, ok := view.Get(chatapp.TimelineEntityChatMessage, id)
	if !ok || entity.Payload == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	pb, ok := entity.Payload.(*chatappv1.ChatMessageEntity)
	if !ok || pb == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	return &chatappv1.ChatMessageEntity{
		MessageId:       pb.GetMessageId(),
		Role:            pb.GetRole(),
		Prompt:          pb.GetPrompt(),
		Text:            pb.GetText(),
		Content:         pb.GetContent(),
		Status:          pb.GetStatus(),
		Streaming:       pb.GetStreaming(),
		Error:           pb.GetError(),
		ParentMessageId: pb.GetParentMessageId(),
		Segment:         pb.GetSegment(),
		SegmentType:     pb.GetSegmentType(),
		Final:           pb.GetFinal(),
	}, true
}

func infoText(data map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}
	if s, ok := data["text"].(string); ok {
		return s
	}
	return ""
}

// Ensure ReasoningPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*ReasoningPlugin)(nil)

// compile-time check for proto usage
var _ proto.Message = (*chatappv1.ChatMessageEntity)(nil)
