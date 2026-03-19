package sqlitetool

import "github.com/go-go-golems/geppetto/pkg/turns"

var (
	KeySQLiteDSN     = turns.DataK[string](PinocchioNamespaceKey, SQLiteDSNValueKey, 1)
	KeySQLitePrompts = turns.DataK[[]string](PinocchioNamespaceKey, SQLitePromptsValueKey, 1)
)
