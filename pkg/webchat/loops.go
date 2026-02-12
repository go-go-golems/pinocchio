package webchat

import (
	"context"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

<<<<<<< HEAD
// ToolCallingLoop wraps geppetto's tool loop.
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
// ToolCallingLoop wraps geppetto's toolhelpers.RunToolCallingLoop.
=======
// ToolCallingLoop wraps geppetto's toolloop.Loop.
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
func ToolCallingLoop(ctx context.Context, eng engine.Engine, t *turns.Turn, reg geptools.ToolRegistry, opts map[string]any) (*turns.Turn, error) {
	loopCfg := toolloop.NewLoopConfig().WithMaxIterations(5)
	toolCfg := geptools.DefaultToolConfig().WithExecutionTimeout(60 * time.Second)
	if v, ok := opts["max_iterations"].(int); ok {
		loopCfg = loopCfg.WithMaxIterations(v)
	}
	if v, ok := opts["timeout_seconds"].(int); ok {
		toolCfg = toolCfg.WithExecutionTimeout(time.Duration(v) * time.Second)
	}
	loop := toolloop.New(
		toolloop.WithEngine(eng),
		toolloop.WithRegistry(reg),
		toolloop.WithLoopConfig(loopCfg),
		toolloop.WithToolConfig(toolCfg),
	)
	return loop.RunLoop(ctx, t)
}
