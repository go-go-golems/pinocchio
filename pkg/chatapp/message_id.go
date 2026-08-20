package chatapp

import (
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const reservedTextMessageIDDelimiter = ":text:"

func defaultMessageIDGenerator() (string, error) {
	return "chat-msg-" + uuid.NewString(), nil
}

func (e *Engine) newMessageID() (string, error) {
	if e == nil || e.messageIDGenerator == nil {
		return "", errors.New("chat message ID generator is not configured")
	}

	id, err := e.messageIDGenerator()
	if err != nil {
		return "", errors.Wrap(err, "generate chat message ID")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("generated chat message ID is empty")
	}
	if strings.Contains(id, reservedTextMessageIDDelimiter) {
		return "", errors.New("generated chat message ID contains reserved text delimiter")
	}
	return id, nil
}
