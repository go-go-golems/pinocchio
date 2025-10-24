//go:build !debugcmds
// +build !debugcmds

package ui

import (
	"github.com/go-go-golems/bobatea/pkg/repl"
	store "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/store"
)

// RegisterDebugCommands is a no-op in production builds; debug REPL commands are gated
// behind the `debugcmds` build tag to avoid unused code lint failures.
func RegisterDebugCommands(*repl.Model, *store.SQLiteStore) {}
