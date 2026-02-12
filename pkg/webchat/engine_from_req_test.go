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

func TestDefaultEngineFromReqBuilder_Chat_ProfilePrecedence(t *testing.T) {
	profiles := newInMemoryProfileRegistry()
	require.NoError(t, profiles.Add(&Profile{Slug: "default"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "existing"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "cookie"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "explicit"}))

	lookup := stubConversationLookup{
		"c1": {ProfileSlug: "existing"},
	}
	b := NewDefaultEngineFromReqBuilder(profiles, lookup)

	t.Run("path beats existing and cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://example.com/chat/explicit", bytes.NewBufferString(`{"prompt":"hi","conv_id":"c1"}`))
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		in, body, err := b.BuildEngineFromReq(req)
		require.NoError(t, err)
		require.NotNil(t, body)
		require.Equal(t, "c1", in.ConvID)
		require.Equal(t, "explicit", in.ProfileSlug)
	})

	t.Run("existing beats cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://example.com/chat", bytes.NewBufferString(`{"prompt":"hi","conv_id":"c1"}`))
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		in, body, err := b.BuildEngineFromReq(req)
		require.NoError(t, err)
		require.NotNil(t, body)
		require.Equal(t, "c1", in.ConvID)
		require.Equal(t, "existing", in.ProfileSlug)
	})

	t.Run("cookie beats default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://example.com/chat", bytes.NewBufferString(`{"prompt":"hi","conv_id":"c2"}`))
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		in, body, err := b.BuildEngineFromReq(req)
		require.NoError(t, err)
		require.NotNil(t, body)
		require.Equal(t, "c2", in.ConvID)
		require.Equal(t, "cookie", in.ProfileSlug)
	})
}

func TestDefaultEngineFromReqBuilder_Chat_GeneratesConvIDWhenMissing(t *testing.T) {
	profiles := newInMemoryProfileRegistry()
	require.NoError(t, profiles.Add(&Profile{Slug: "default"}))
	b := NewDefaultEngineFromReqBuilder(profiles, stubConversationLookup{})

	req := httptest.NewRequest(http.MethodPost, "http://example.com/chat", bytes.NewBufferString(`{"prompt":"hi"}`))
	in, body, err := b.BuildEngineFromReq(req)
	require.NoError(t, err)
	require.NotNil(t, body)
	require.NotEmpty(t, in.ConvID)
	require.Equal(t, in.ConvID, body.ConvID)
	_, parseErr := uuid.Parse(in.ConvID)
	require.NoError(t, parseErr)
}

func TestDefaultEngineFromReqBuilder_WS_ProfilePrecedence(t *testing.T) {
	profiles := newInMemoryProfileRegistry()
	require.NoError(t, profiles.Add(&Profile{Slug: "default"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "cookie"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "existing"}))
	require.NoError(t, profiles.Add(&Profile{Slug: "explicit"}))

	lookup := stubConversationLookup{
		"c1": {ProfileSlug: "existing"},
	}
	b := NewDefaultEngineFromReqBuilder(profiles, lookup)

	t.Run("query beats cookie and existing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ws?conv_id=c1&profile=explicit", nil)
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		in, body, err := b.BuildEngineFromReq(req)
		require.NoError(t, err)
		require.Nil(t, body)
		require.Equal(t, "c1", in.ConvID)
		require.Equal(t, "explicit", in.ProfileSlug)
	})

	t.Run("cookie beats existing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ws?conv_id=c1", nil)
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "cookie"})
		in, _, err := b.BuildEngineFromReq(req)
		require.NoError(t, err)
		require.Equal(t, "cookie", in.ProfileSlug)
	})

	t.Run("existing beats default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ws?conv_id=c1", nil)
		in, _, err := b.BuildEngineFromReq(req)
		require.NoError(t, err)
		require.Equal(t, "existing", in.ProfileSlug)
	})

	t.Run("missing conv_id is a 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
		_, _, err := b.BuildEngineFromReq(req)
		var rbe *RequestBuildError
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
