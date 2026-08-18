package appserver

import (
	"context"
	"strings"
	"time"

	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
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

// HydrationStoreFactory constructs the timeline store after the app server
// has registered all protobuf schemas.
type HydrationStoreFactory func(context.Context, *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error)

func WithHydrationStoreSpec(spec serverkit.StoreSpec) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.timelineSpec = spec
	}
}

func WithHydrationStoreFactory(factory HydrationStoreFactory) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.hydrationFactory = factory
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
