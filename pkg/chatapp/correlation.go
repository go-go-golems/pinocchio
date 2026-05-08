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
