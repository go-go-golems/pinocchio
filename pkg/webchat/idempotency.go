package webchat

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

func idempotencyKeyFromRequest(r *http.Request, body *ChatRequestBody) string {
	var key string
	if r != nil {
		key = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			key = strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
		}
	}
	if key == "" && body != nil {
		key = strings.TrimSpace(body.IdempotencyKey)
	}
	if key == "" {
		key = uuid.NewString()
	}
	return key
}
