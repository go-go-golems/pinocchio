package chatapp

import (
	"math"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
)

func CorrelationInfoFromEvent(event gepevents.Event) *chatappv1.CorrelationInfo {
	if event == nil {
		return nil
	}
	correlated, ok := event.(gepevents.CorrelatedEvent)
	if !ok {
		return nil
	}
	return CorrelationInfoFromGeppetto(correlated.Correlation())
}

func correlationInfoFromEvent(event gepevents.Event) *chatappv1.CorrelationInfo {
	return CorrelationInfoFromEvent(event)
}

func CorrelationInfoFromGeppetto(corr gepevents.Correlation) *chatappv1.CorrelationInfo {
	return &chatappv1.CorrelationInfo{
		SessionId:            corr.SessionID,
		RunId:                corr.RunID,
		InferenceId:          corr.InferenceID,
		TurnId:               corr.TurnID,
		ProviderCallId:       corr.ProviderCallID,
		ProviderCallIndex:    corr.ProviderCallIndex,
		Provider:             corr.Provider,
		Model:                corr.Model,
		ResponseId:           corr.ResponseID,
		ItemId:               corr.ItemID,
		OutputIndex:          cloneInt32Ptr(corr.OutputIndex),
		SummaryIndex:         cloneInt32Ptr(corr.SummaryIndex),
		ChoiceIndex:          cloneInt32Ptr(corr.ChoiceIndex),
		ContentBlockIndex:    cloneInt32Ptr(corr.ContentBlockIndex),
		SegmentId:            corr.SegmentID,
		SegmentIndex:         corr.SegmentIndex,
		SegmentType:          corr.SegmentType,
		StreamKind:           corr.StreamKind,
		ToolCallId:           corr.ToolCallID,
		ToolCallIndex:        cloneInt32Ptr(corr.ToolCallIndex),
		CorrelationKey:       corr.CorrelationKey,
		ParentCorrelationKey: corr.ParentCorrelationKey,
	}
}

func UsageInfoFromGeppetto(usage *gepevents.Usage) *chatappv1.UsageInfo {
	if usage == nil {
		return nil
	}
	return &chatappv1.UsageInfo{
		InputTokens:              intToInt32Saturating(usage.InputTokens),
		OutputTokens:             intToInt32Saturating(usage.OutputTokens),
		CachedTokens:             intToInt32Saturating(usage.CachedTokens),
		CacheCreationInputTokens: intToInt32Saturating(usage.CacheCreationInputTokens),
		CacheReadInputTokens:     intToInt32Saturating(usage.CacheReadInputTokens),
	}
}

func usageInfoFromGeppetto(usage *gepevents.Usage) *chatappv1.UsageInfo {
	return UsageInfoFromGeppetto(usage)
}

func intToInt32Saturating(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

func cloneInt32Ptr(v *int32) *int32 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func CloneCorrelationInfo(corr *chatappv1.CorrelationInfo) *chatappv1.CorrelationInfo {
	if corr == nil {
		return nil
	}
	out := *corr
	out.OutputIndex = cloneInt32Ptr(corr.OutputIndex)
	out.SummaryIndex = cloneInt32Ptr(corr.SummaryIndex)
	out.ChoiceIndex = cloneInt32Ptr(corr.ChoiceIndex)
	out.ContentBlockIndex = cloneInt32Ptr(corr.ContentBlockIndex)
	out.ToolCallIndex = cloneInt32Ptr(corr.ToolCallIndex)
	return &out
}

func MergeCorrelationInfo(existing, update *chatappv1.CorrelationInfo) *chatappv1.CorrelationInfo {
	if existing == nil {
		return CloneCorrelationInfo(update)
	}
	if update == nil {
		return CloneCorrelationInfo(existing)
	}
	out := CloneCorrelationInfo(existing)
	if update.SessionId != "" {
		out.SessionId = update.SessionId
	}
	if update.RunId != "" {
		out.RunId = update.RunId
	}
	if update.InferenceId != "" {
		out.InferenceId = update.InferenceId
	}
	if update.TurnId != "" {
		out.TurnId = update.TurnId
	}
	if update.ProviderCallId != "" {
		out.ProviderCallId = update.ProviderCallId
	}
	if update.ProviderCallIndex != 0 {
		out.ProviderCallIndex = update.ProviderCallIndex
	}
	if update.Provider != "" {
		out.Provider = update.Provider
	}
	if update.Model != "" {
		out.Model = update.Model
	}
	if update.ResponseId != "" {
		out.ResponseId = update.ResponseId
	}
	if update.ItemId != "" {
		out.ItemId = update.ItemId
	}
	if update.OutputIndex != nil {
		out.OutputIndex = cloneInt32Ptr(update.OutputIndex)
	}
	if update.SummaryIndex != nil {
		out.SummaryIndex = cloneInt32Ptr(update.SummaryIndex)
	}
	if update.ChoiceIndex != nil {
		out.ChoiceIndex = cloneInt32Ptr(update.ChoiceIndex)
	}
	if update.ContentBlockIndex != nil {
		out.ContentBlockIndex = cloneInt32Ptr(update.ContentBlockIndex)
	}
	if update.SegmentId != "" {
		out.SegmentId = update.SegmentId
	}
	if update.SegmentIndex != 0 {
		out.SegmentIndex = update.SegmentIndex
	}
	if update.SegmentType != "" {
		out.SegmentType = update.SegmentType
	}
	if update.StreamKind != "" {
		out.StreamKind = update.StreamKind
	}
	if update.ToolCallId != "" {
		out.ToolCallId = update.ToolCallId
	}
	if update.ToolCallIndex != nil {
		out.ToolCallIndex = cloneInt32Ptr(update.ToolCallIndex)
	}
	if update.CorrelationKey != "" {
		out.CorrelationKey = update.CorrelationKey
	}
	if update.ParentCorrelationKey != "" {
		out.ParentCorrelationKey = update.ParentCorrelationKey
	}
	return out
}
