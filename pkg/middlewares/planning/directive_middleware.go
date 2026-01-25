package planning

import (
	"context"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/pkg/errors"
)

const (
	directiveStartMarker = "\n\n<!-- pinocchio:planning-directive:start -->\n"
	directiveEndMarker   = "\n<!-- pinocchio:planning-directive:end -->\n"
)

// NewDirectiveMiddleware injects the planner's final directive into the first system block.
//
// This middleware is intentionally idempotent: it rewrites the directive section between markers.
func NewDirectiveMiddleware() middleware.Middleware {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
			if t == nil {
				t = &turns.Turn{}
			}

			directive, ok, err := KeyDirective.Get(t.Data)
			if err != nil || !ok {
				return next(ctx, t)
			}
			directive = strings.TrimSpace(directive)
			if directive == "" {
				return next(ctx, t)
			}

			ensureFirstSystemBlock(t)
			idx := firstSystemBlockIndex(t)
			if idx < 0 {
				return next(ctx, t)
			}

			if t.Blocks[idx].Payload == nil {
				t.Blocks[idx].Payload = map[string]any{}
			}
			existing, _ := t.Blocks[idx].Payload[turns.PayloadKeyText].(string)
			existing = removeDirectiveSection(existing)
			if existing != "" {
				existing = strings.TrimRight(existing, "\n")
			}
			out := existing + directiveStartMarker + directive + directiveEndMarker
			t.Blocks[idx].Payload[turns.PayloadKeyText] = out

			return next(ctx, t)
		}
	}
}

func ensureFirstSystemBlock(t *turns.Turn) {
	if t == nil {
		return
	}
	if firstSystemBlockIndex(t) >= 0 {
		return
	}
	b := turns.NewSystemTextBlock("")
	t.Blocks = append([]turns.Block{b}, t.Blocks...)
}

func firstSystemBlockIndex(t *turns.Turn) int {
	if t == nil {
		return -1
	}
	for i, b := range t.Blocks {
		if b.Kind == turns.BlockKindSystem {
			return i
		}
	}
	return -1
}

func removeDirectiveSection(s string) string {
	start := strings.Index(s, directiveStartMarker)
	if start < 0 {
		return s
	}
	end := strings.Index(s[start+len(directiveStartMarker):], directiveEndMarker)
	if end < 0 {
		// Malformed markers: drop everything from start.
		return strings.TrimSpace(s[:start])
	}
	endAbs := start + len(directiveStartMarker) + end + len(directiveEndMarker)
	out := s[:start] + s[endAbs:]
	return strings.TrimSpace(out)
}

// ErrPlannerOutputEmpty is returned when the planner call produces no assistant text payload.
var ErrPlannerOutputEmpty = errors.New("planner output was empty")
