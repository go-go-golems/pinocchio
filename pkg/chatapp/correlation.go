package chatapp

import (
	"math"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	"google.golang.org/protobuf/proto"
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
		SessionId:      corr.SessionID,
		RunId:          corr.RunID,
		TurnId:         corr.TurnID,
		ProviderCallId: corr.ProviderCallID,
		SegmentId:      corr.SegmentID,
		ToolCallId:     corr.ToolCallID,
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

func CloneCorrelationInfo(corr *chatappv1.CorrelationInfo) *chatappv1.CorrelationInfo {
	if corr == nil {
		return nil
	}
	return proto.Clone(corr).(*chatappv1.CorrelationInfo)
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
	if update.TurnId != "" {
		out.TurnId = update.TurnId
	}
	if update.ProviderCallId != "" {
		out.ProviderCallId = update.ProviderCallId
	}
	if update.SegmentId != "" {
		out.SegmentId = update.SegmentId
	}
	if update.ToolCallId != "" {
		out.ToolCallId = update.ToolCallId
	}
	return out
}
