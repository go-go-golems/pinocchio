package oauthprofiles

import (
	"context"
	"errors"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	geppettoauth "github.com/go-go-golems/geppetto/pkg/steps/ai/credentials/oauth"
)

// Refresher adapts Geppetto's profile-agnostic OAuth protocol client to its
// host-owned credentials.Refresher contract.
type Refresher struct {
	client *geppettoauth.Client
}

var _ credentials.Refresher = (*Refresher)(nil)

// NewRefresher binds one validated OAuth protocol client to a Pinocchio
// profile. The client contains no access or refresh material.
func NewRefresher(client *geppettoauth.Client) (*Refresher, error) {
	if client == nil {
		return nil, errors.New("OAuth protocol client is required")
	}
	return &Refresher{client: client}, nil
}

// Refresh ignores the non-secret request identity because YAMLStore already
// validates it before persistence; the protocol client refreshes the supplied
// host-owned previous credential and returns no token material in errors.
func (r *Refresher) Refresh(ctx context.Context, _ credentials.Request, previous credentials.Credential) (credentials.Credential, error) {
	if r == nil || r.client == nil {
		return credentials.Credential{}, errors.New("OAuth refresher is unavailable")
	}
	return r.client.Refresh(ctx, previous)
}
