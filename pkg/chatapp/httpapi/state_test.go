package httpapi

import (
	"context"
	"testing"

	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	"github.com/stretchr/testify/require"
)

func TestContextFieldMakesABlockWithinItsLimit(t *testing.T) {
	blocks := ContextField(10)
	got, err := blocks(context.Background(), sessions.Info{}, MessageRequest{Raw: []byte(`{"prompt":"p","context":"[state]"}`)})
	require.NoError(t, err)
	require.Len(t, got, 1)
	got, err = blocks(context.Background(), sessions.Info{}, MessageRequest{Raw: []byte(`{"prompt":"p"}`)})
	require.NoError(t, err)
	require.Empty(t, got)
	_, err = blocks(context.Background(), sessions.Info{}, MessageRequest{Raw: []byte(`{"context":"more than ten bytes"}`)})
	var coded chatapp.CodedError
	require.ErrorAs(t, err, &coded)
	require.Equal(t, chatapp.CodePromptTooLarge, coded.Code())
}
