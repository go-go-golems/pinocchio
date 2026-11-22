package sessionguard

import (
	"context"
	"errors"
	"testing"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	"github.com/stretchr/testify/require"
)

func ownerOnly(_ context.Context, p Principal, _ Action, s *sessions.Info) error {
	if s == nil || s.Owner == p.ID {
		return nil
	}
	return errors.New("not yours")
}

func TestASessionCanOnlyDoWhatItWasOpenedFor(t *testing.T) {
	ctx := context.Background()
	g := New(sessions.NewMemoryStore(), ownerOnly)
	alice := g.For(Principal{ID: "alice"})
	_, err := alice.Create(ctx, sessions.Info{ID: "s1", Owner: "mallory"})
	require.NoError(t, err)

	read, err := alice.Open(ctx, "s1", ActionRead)
	require.NoError(t, err)
	require.Equal(t, "alice", read.Info().Owner, "Create sets the owner to the principal, whatever the request says")
	require.ErrorAs(t, read.Delete(ctx), new(ForbiddenError), "a session opened to read cannot be deleted")
	require.ErrorAs(t, read.Update(ctx, func(*sessions.Info) error { return nil }), new(ForbiddenError))

	send, err := alice.Open(ctx, "s1", ActionSend)
	require.NoError(t, err)
	require.NoError(t, send.Update(ctx, func(info *sessions.Info) error { info.Owner = "mallory"; info.Title = "t"; return nil }))
	stored, err := g.Unchecked().Get(ctx, "s1")
	require.NoError(t, err)
	require.Equal(t, "alice", stored.Owner, "an update cannot change the owner")
	require.Equal(t, "t", stored.Title)

	_, err = g.For(Principal{ID: "bob"}).Open(ctx, "s1", ActionRead)
	require.ErrorAs(t, err, new(ForbiddenError))

	del, err := alice.Open(ctx, "s1", ActionDelete)
	require.NoError(t, err)
	require.NoError(t, del.Delete(ctx))
}

func TestAListingIsThePrincipalsOwnWhateverTheFilterSays(t *testing.T) {
	ctx := context.Background()
	g := New(sessions.NewMemoryStore(), ownerOnly)
	_, err := g.For(Principal{ID: "alice"}).Create(ctx, sessions.Info{ID: "a"})
	require.NoError(t, err)
	_, err = g.For(Principal{ID: "bob"}).Create(ctx, sessions.Info{ID: "b"})
	require.NoError(t, err)

	list, err := g.For(Principal{ID: "bob"}).List(ctx, sessions.Filter{AllOwners: true, Owner: "alice"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "b", list[0].ID)
	anon, err := g.For(Principal{}).List(ctx, sessions.Filter{AllOwners: true})
	require.NoError(t, err)
	require.Empty(t, anon)
}

func TestACreateHookCannotChangeTheIDOrTheOwner(t *testing.T) {
	ctx := context.Background()
	hook := func(_ context.Context, _ Principal, info *sessions.Info) error {
		info.ID, info.Owner, info.Title = "other", "mallory", "from the hook"
		return nil
	}
	g := New(sessions.NewMemoryStore(), ownerOnly, WithCreateHook(hook))
	s, err := g.For(Principal{ID: "alice"}).Create(ctx, sessions.Info{ID: "s1"})
	require.NoError(t, err)
	require.Equal(t, "s1", s.ID())
	require.Equal(t, "alice", s.Info().Owner)
	require.Equal(t, "from the hook", s.Info().Title)

	refuse := New(sessions.NewMemoryStore(), ownerOnly, WithCreateHook(func(context.Context, Principal, *sessions.Info) error { return errors.New("no") }))
	_, err = refuse.For(Principal{ID: "alice"}).Create(ctx, sessions.Info{ID: "s1"})
	require.ErrorAs(t, err, new(CreateRejectedError))
}
