package appserver

import (
	"strings"
	"time"

	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

type Option func(*Server)

func WithDefaultProfile(profile string) Option {
	return func(s *Server) {
		s.defaultProfile = strings.TrimSpace(profile)
	}
}

func WithChunkDelay(delay time.Duration) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.chunkDelay = delay
	}
}

func WithSQLiteDSN(dsn string) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.sqliteDSN = strings.TrimSpace(dsn)
	}
}

func WithSQLiteDBPath(path string) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.sqliteDBPath = strings.TrimSpace(path)
	}
}

func WithRuntimeResolver(resolver RuntimeResolver) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.runtimeResolver = resolver
	}
}

func WithTurnStore(store chatstore.TurnStore) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.turnStore = store
	}
}

func WithTurnsDBPath(path string) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.turnsDBPath = strings.TrimSpace(path)
	}
}

func WithChatPlugins(features ...chatapp.ChatPlugin) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		for _, feature := range features {
			if feature != nil {
				s.chatPlugins = append(s.chatPlugins, feature)
			}
		}
	}
}

func WithFrontendToolManager(manager *frontendtools.Manager) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.frontendToolManager = manager
	}
}
