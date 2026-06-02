package appserver

import (
	"net/http"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	serverkit.WriteJSON(w, status, payload)
}
