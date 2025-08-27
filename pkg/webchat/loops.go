package webchat

import (
	"context"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

// ToolCallingLoop wraps geppetto's toolhelpers.RunToolCallingLoop.
func ToolCallingLoop(ctx context.Context, eng engine.Engine, t *turns.Turn, reg geptools.ToolRegistry, opts map[string]any) (*turns.Turn, error) {
	cfg := toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60 * time.Second)
	if v, ok := opts["max_iterations"].(int); ok {
		cfg = cfg.WithMaxIterations(v)
	}
	if v, ok := opts["timeout_seconds"].(int); ok {
		cfg = cfg.WithTimeout(time.Duration(v) * time.Second)
	}
	return toolhelpers.RunToolCallingLoop(ctx, eng, t, reg, cfg)
}
