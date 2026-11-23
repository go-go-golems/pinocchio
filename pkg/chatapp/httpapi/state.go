package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/pkg/errors"
)

// WithStateDir keeps the server's state in dir, which is created if needed:
// sessions.db (the session list), turns.db (conversation history) and
// timeline.db (the timeline a snapshot is read from), all SQLite. Stores set
// with WithSessionStore, WithTurnStore or WithHydrationStoreSpec/Factory take
// precedence over the file of the same role. Close closes what it opened.
func WithStateDir(dir string) Option {
	return func(s *Server) { s.stateDir = dir }
}

// openStateDir opens the state directory's stores; it returns the session
// store when needSessions (New assigns it: only New and WithSessionStore set
// the store, see guard_test.go).
func (s *Server) openStateDir(needSessions bool) (sessions.Store, error) {
	if s.stateDir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(s.stateDir, 0o700); err != nil {
		return nil, errors.Wrap(err, "httpapi: state directory")
	}
	var sessionStore sessions.Store
	if needSessions {
		store, err := sessions.NewSQLiteStore("file:" + filepath.Join(s.stateDir, "sessions.db") + "?_busy_timeout=5000&_journal_mode=WAL")
		if err != nil {
			return nil, errors.Wrap(err, "httpapi: open session store")
		}
		s.stateClosers = append(s.stateClosers, store.Close)
		sessionStore = store
	}
	if s.turnStore == nil {
		path := filepath.Join(s.stateDir, "turns.db")
		store, err := chatstore.NewSQLiteTurnStore(path)
		if err != nil {
			return nil, errors.Wrap(err, "httpapi: open turn store")
		}
		s.stateClosers = append(s.stateClosers, store.Close)
		s.turnStore = store
		if s.turnsDBPath == "" {
			s.turnsDBPath = path
		}
	}
	if s.hydrationFactory == nil && s.hydrationSpec.Backend == serverkit.StoreBackendMemory {
		s.hydrationSpec = serverkit.StoreSpec{Backend: serverkit.StoreBackendSQLite, Path: filepath.Join(s.stateDir, "timeline.db")}
	}
	return sessionStore, nil
}

func (s *Server) closeState() error {
	var first error
	for i := len(s.stateClosers) - 1; i >= 0; i-- {
		if err := s.stateClosers[i](); err != nil && first == nil {
			first = err
		}
	}
	s.stateClosers = nil
	return first
}

// ContextField is a BlocksFunc for a message body's "context" string: a user
// block after the person's message (the page's state, for example). The block
// stays in the conversation, which is append-only: a page should send what
// changed. More than maxBytes is refused with prompt_too_large; maxBytes <= 0
// means no limit.
func ContextField(maxBytes int) BlocksFunc {
	return func(_ context.Context, _ sessions.Info, msg MessageRequest) ([]turns.Block, error) {
		if len(msg.Raw) == 0 {
			return nil, nil
		}
		var extra struct {
			Context string `json:"context"`
		}
		if err := json.Unmarshal(msg.Raw, &extra); err != nil {
			return nil, Coded(CodeInvalidRequest, "the message body is not a JSON object: "+err.Error())
		}
		if strings.TrimSpace(extra.Context) == "" {
			return nil, nil
		}
		if maxBytes > 0 && len(extra.Context) > maxBytes {
			return nil, Coded(chatapp.CodePromptTooLarge, fmt.Sprintf("a message's context is at most %d bytes", maxBytes))
		}
		return []turns.Block{turns.NewUserTextBlock(extra.Context)}, nil
	}
}
