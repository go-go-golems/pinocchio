// Package budget has round guards that stop a run when a token budget is used
// up. A guard runs before every model call (geppetto's toolloop.RoundGuard),
// so a run stops between rounds, before the call that would exceed the
// budget, and fails with code "quota_exceeded".
package budget

import (
	"context"
	"errors"
	"fmt"

	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
)

// CodeQuotaExceeded is the run failure code of an exhausted budget.
const CodeQuotaExceeded = "quota_exceeded"

// QuotaExceededError stops a run whose session used its budget. It
// implements chatapp.CodedError.
type QuotaExceededError struct {
	Used, Limit int
}

func (e QuotaExceededError) Error() string {
	return fmt.Sprintf("the session used %d of its %d tokens", e.Used, e.Limit)
}

// Code is CodeQuotaExceeded.
func (QuotaExceededError) Code() string { return CodeQuotaExceeded }

// SessionTokens returns a round guard that stops a run once its session used
// limit tokens (sessions.Usage.Tokens: input + output over all runs). The
// session is the run's geppetto session id, which chatapp sets to the chatapp
// session id. The usage must be recorded into store (sessions.RecordUsage;
// httpapi does it). A session missing from the store is not limited; other
// store errors stop the run. limit <= 0 means no limit.
func SessionTokens(store sessions.Store, limit int) toolloop.RoundGuard {
	return func(ctx context.Context, _ *turns.Turn, _ int) error {
		if limit <= 0 || store == nil {
			return nil
		}
		sid := gepsession.SessionIDFromContext(ctx)
		if sid == "" {
			return nil
		}
		info, err := store.Get(ctx, sid)
		if errors.Is(err, sessions.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("budget: read session %s: %w", sid, err)
		}
		if used := info.Usage.Tokens(); used >= limit {
			return QuotaExceededError{Used: used, Limit: limit}
		}
		return nil
	}
}

// All runs guards in order and returns the first error.
func All(guards ...toolloop.RoundGuard) toolloop.RoundGuard {
	return func(ctx context.Context, t *turns.Turn, round int) error {
		for _, g := range guards {
			if g == nil {
				continue
			}
			if err := g(ctx, t, round); err != nil {
				return err
			}
		}
		return nil
	}
}
