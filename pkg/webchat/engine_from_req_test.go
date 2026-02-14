package webchat

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubConversationLookup map[string]*Conversation

func (s stubConversationLookup) GetConversation(convID string) (*Conversation, bool) {
	c, ok := s[convID]
	return c, ok
}

func TestDefaultConversationRequestResolver_Chat_ProfilePrecedence(t *testing.T) {
	profiles := newInMemoryProfileRegistry()
	require.NoError(t, profiles.Add(&Profile{Slug: "default"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "existing"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "cookie"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "explicit"}))

	lookup := stubConversationLookup{
		"c1": {ProfileSlug: "existing"},
	}
	b := NewDefaultConversationRequestResolver(profiles, lookup)

	t.Run("path beats existing and cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://example.com/chat/explicit", bytes.NewBufferString(`{"prompt":"hi","conv_id":"c1"}`))
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		plan, err := b.Resolve(req)
		require.NoError(t, err)
		require.Equal(t, "c1", plan.ConvID)
		require.Equal(t, "explicit", plan.RuntimeKey)
		require.Equal(t, "hi", plan.Prompt)
	})

	t.Run("existing beats cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://example.com/chat", bytes.NewBufferString(`{"prompt":"hi","conv_id":"c1"}`))
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		plan, err := b.Resolve(req)
		require.NoError(t, err)
		require.Equal(t, "c1", plan.ConvID)
		require.Equal(t, "existing", plan.RuntimeKey)
		require.Equal(t, "hi", plan.Prompt)
	})

	t.Run("cookie beats default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://example.com/chat", bytes.NewBufferString(`{"prompt":"hi","conv_id":"c2"}`))
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		plan, err := b.Resolve(req)
		require.NoError(t, err)
		require.Equal(t, "c2", plan.ConvID)
		require.Equal(t, "cookie", plan.RuntimeKey)
		require.Equal(t, "hi", plan.Prompt)
	})
}

func TestDefaultConversationRequestResolver_Chat_GeneratesConvIDWhenMissing(t *testing.T) {
	profiles := newInMemoryProfileRegistry()
	require.NoError(t, profiles.Add(&Profile{Slug: "default"}))
	b := NewDefaultConversationRequestResolver(profiles, stubConversationLookup{})

	req := httptest.NewRequest(http.MethodPost, "http://example.com/chat", bytes.NewBufferString(`{"prompt":"hi"}`))
	plan, err := b.Resolve(req)
	require.NoError(t, err)
	require.NotEmpty(t, plan.ConvID)
	require.Equal(t, "hi", plan.Prompt)
	_, parseErr := uuid.Parse(plan.ConvID)
	require.NoError(t, parseErr)
}

func TestDefaultConversationRequestResolver_WS_ProfilePrecedence(t *testing.T) {
	profiles := newInMemoryProfileRegistry()
	require.NoError(t, profiles.Add(&Profile{Slug: "default"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "cookie"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "existing"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "explicit"}))

	lookup := stubConversationLookup{
		"c1": {ProfileSlug: "existing"},
	}
	b := NewDefaultConversationRequestResolver(profiles, lookup)

	t.Run("query beats cookie and existing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ws?conv_id=c1&profile=explicit", nil)
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		plan, err := b.Resolve(req)
		require.NoError(t, err)
		require.Equal(t, "c1", plan.ConvID)
		require.Equal(t, "explicit", plan.RuntimeKey)
		require.Empty(t, plan.Prompt)
	})

	t.Run("cookie beats existing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ws?conv_id=c1", nil)
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		plan, err := b.Resolve(req)
		require.NoError(t, err)
		require.Equal(t, "cookie", plan.RuntimeKey)
	})

	t.Run("existing beats default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ws?conv_id=c1", nil)
		plan, err := b.Resolve(req)
		require.NoError(t, err)
		require.Equal(t, "existing", plan.RuntimeKey)
	})

	t.Run("missing conv_id is a 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
		_, err := b.Resolve(req)
		var rbe *RequestResolutionError
		require.ErrorAs(t, err, &rbe)
		require.Equal(t, http.StatusBadRequest, rbe.Status)
	})
}

func TestBuildConfig_RejectsEngineOverridesWhenDisallowed(t *testing.T) {
	r := &Router{
		parsed:   &values.Values{},
		profiles: newInMemoryProfileRegistry(),
	}
	require.NoError(t, r.profiles.Add(&Profile{Slug: "default", AllowOverrides: false}))

	_, err := r.BuildConfig("default", map[string]any{"system_prompt": "x"})
	require.Error(t, err)
}
