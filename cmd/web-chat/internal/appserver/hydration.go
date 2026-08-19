package appserver

import (
	"context"
	"fmt"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

func newHydrationStore(s *Server, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error) {
	if s == nil || reg == nil {
		return nil, nil, fmt.Errorf("app server or schema registry is nil")
	}
	if s.hydrationFactory != nil {
		return s.hydrationFactory(context.Background(), reg)
	}
	return serverkit.OpenHydrationStore(context.Background(), s.timelineSpec, reg)
}
