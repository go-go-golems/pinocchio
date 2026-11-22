package chatapp

import (
	"context"
	"errors"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/toolblocks"
)

// Run outcome codes published in ChatRunFailed.code and ChatRunFinished.reason.
const (
	CodeMaxRounds           = "max_rounds"
	CodeAuthenticationError = "authentication_error"
	CodeRateLimited         = "rate_limited"
	CodePromptTooLarge      = "prompt_too_large"
	CodeUpstreamError       = "upstream_error"
)

// CodedError is an error that names its own outcome code. A RoundGuard that
// stops a run (for example a token budget) returns one so the run fails with
// that code instead of a classified provider code.
type CodedError interface {
	error
	Code() string
}

// runOutcome is how a run ended, decided in one place from the loop's error.
type runOutcome struct {
	kind      outcomeKind
	code      string // failed runs
	truncated bool   // finished runs that hit the round limit and answered
	reason    string // why a truncated run ended
}

type outcomeKind int

const (
	outcomeFinished outcomeKind = iota
	outcomeStopped
	outcomeFailed
)

// decideOutcome maps the loop's error to an outcome:
//
//   - no error: finished
//   - context cancelled (Stop, or replaced by a new prompt): stopped
//   - round limit reached and the final answer given: finished, truncated
//   - round limit reached without a final answer: failed, max_rounds
//   - an error with a Code() method (e.g. a RoundGuard's): failed with that code
//   - anything else: failed with the classifier's code
func (e *Engine) decideOutcome(ctx context.Context, err error) runOutcome {
	var limit *toolloop.MaxIterationsError
	var coded CodedError
	switch {
	case err == nil:
		return runOutcome{kind: outcomeFinished}
	case errors.Is(err, context.Canceled) || ctx.Err() != nil:
		return runOutcome{kind: outcomeStopped}
	case errors.As(err, &limit) && limit.Answered:
		// geppetto sets Answered only when the final answer succeeded; a failed
		// final answer wraps its cause next to a MaxIterationsError{Answered: false}.
		return runOutcome{kind: outcomeFinished, truncated: true, reason: CodeMaxRounds}
	case errors.As(err, &coded) && strings.TrimSpace(coded.Code()) != "":
		return runOutcome{kind: outcomeFailed, code: coded.Code()}
	case errors.As(err, &limit):
		return runOutcome{kind: outcomeFailed, code: CodeMaxRounds}
	default:
		return runOutcome{kind: outcomeFailed, code: e.classify(err)}
	}
}

// classify applies the configured classifier, or ClassifyError.
func (e *Engine) classify(err error) string {
	if e.classifyError != nil {
		return e.classifyError(err)
	}
	return ClassifyError(err)
}

// ClassifyError is the default classifier: it maps provider errors to stable
// codes by their text, because geppetto's OpenAI-style engines return untyped
// errors. Replace it with WithErrorClassifier.
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}
	m := strings.ToLower(err.Error())
	switch {
	case strings.Contains(m, "401") || strings.Contains(m, "403") || strings.Contains(m, "unauthorized") ||
		strings.Contains(m, "credential") || strings.Contains(m, "authentication"):
		return CodeAuthenticationError
	case strings.Contains(m, "429") || strings.Contains(m, "rate limit") || strings.Contains(m, "rate_limit"):
		return CodeRateLimited
	case strings.Contains(m, "context length") || strings.Contains(m, "too long") || strings.Contains(m, "too large") ||
		strings.Contains(m, "maximum context"):
		return CodePromptTooLarge
	}
	return CodeUpstreamError
}

// failureText is the error text published for a failed run.
func (e *Engine) failureText(code string, err error) string {
	if e.errorText != nil {
		return e.errorText(code, err)
	}
	if err == nil {
		return code
	}
	return err.Error()
}

// closeUnansweredToolCalls appends an error result for every tool call in t
// that has no result, so the turn is a valid provider request again, and
// returns how many it closed. It uses geppetto's own pending-call detection.
func closeUnansweredToolCalls(t *turns.Turn, text string) int {
	if t == nil {
		return 0
	}
	pending := toolblocks.ExtractPendingToolCalls(t)
	if len(pending) == 0 {
		return 0
	}
	results := make([]toolblocks.ToolResult, 0, len(pending))
	for _, c := range pending {
		results = append(results, toolblocks.ToolResult{ID: c.ID, Error: text})
	}
	toolblocks.AppendToolResultsBlocks(t, results)
	return len(results)
}
