package chatapp

import (
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const (
	userMessageIDSuffix         = "-user"
	textMessageIDDelimiter      = ":text:"
	reasoningMessageIDDelimiter = ":thinking:"
	warningMessageIDSuffix      = ":warning"
)

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
	if reserved := reservedDerivedMessageIDNamespace(id); reserved != "" {
		return "", errors.Errorf("generated chat message ID occupies reserved derived namespace %q", reserved)
	}
	return id, nil
}

func reservedDerivedMessageIDNamespace(id string) string {
	for _, delimiter := range []string{textMessageIDDelimiter, reasoningMessageIDDelimiter} {
		if strings.Contains(id, delimiter) {
			return delimiter
		}
	}
	for _, suffix := range []string{userMessageIDSuffix, warningMessageIDSuffix} {
		if strings.HasSuffix(id, suffix) {
			return suffix
		}
	}
	return ""
}
