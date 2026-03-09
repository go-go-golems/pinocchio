package ui

import (
	"errors"
	"testing"

	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
)

func TestBackendWaitResultMsgReturnsErrorMsg(t *testing.T) {
	err := errors.New("provider failed")
	msg := backendWaitResultMsg(err)

	errorMsg, ok := msg.(boba_chat.ErrorMsg)
	if !ok {
		t.Fatalf("msg type = %T, want boba_chat.ErrorMsg", msg)
	}
	if got, want := error(errorMsg).Error(), err.Error(); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestBackendWaitResultMsgReturnsFinishedMsgOnSuccess(t *testing.T) {
	msg := backendWaitResultMsg(nil)
	if _, ok := msg.(boba_chat.BackendFinishedMsg); !ok {
		t.Fatalf("msg type = %T, want boba_chat.BackendFinishedMsg", msg)
	}
}
