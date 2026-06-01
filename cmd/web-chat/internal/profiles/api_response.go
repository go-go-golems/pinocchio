package profiles

import (
	"encoding/json"
	"errors"
	"net/http"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
)

func writeProfileRegistryError(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, gepprofiles.ErrProfileNotFound):
		http.Error(w, "profile not found", http.StatusNotFound)
	case errors.Is(err, gepprofiles.ErrRegistryNotFound):
		http.Error(w, "registry not found", http.StatusNotFound)
	case errors.Is(err, gepprofiles.ErrValidation):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, gepprofiles.ErrReadOnlyStore):
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
	}
}

func writeJSONResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if status > 0 {
		w.WriteHeader(status)
	}
	_ = json.NewEncoder(w).Encode(payload)
}
