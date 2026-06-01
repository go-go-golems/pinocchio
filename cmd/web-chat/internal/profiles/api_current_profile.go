package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
)

func registerCurrentProfileHandler(mux *http.ServeMux, profileRegistry gepprofiles.Registry, opts APIOptions) {
	mux.HandleFunc("/api/chat/profile", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			slug := gepprofiles.EngineProfileSlug("")
			registrySlug := opts.DefaultRegistrySlug
			if ck, err := req.Cookie(opts.CurrentProfileCookieName); err == nil && ck != nil {
				if parsedRegistry, parsedProfile, ok := parseCurrentProfileCookieValue(strings.TrimSpace(ck.Value)); ok && profileExists(req.Context(), profileRegistry, parsedRegistry, parsedProfile) {
					registrySlug = parsedRegistry
					slug = parsedProfile
				} else if parsed, err := gepprofiles.ParseEngineProfileSlug(strings.TrimSpace(ck.Value)); err == nil && profileExists(req.Context(), profileRegistry, opts.DefaultRegistrySlug, parsed) {
					slug = parsed
				}
			}
			if slug.IsZero() {
				defaultSlug, err := resolveDefaultEngineProfileSlug(req.Context(), profileRegistry, registrySlug)
				if err != nil {
					http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
					return
				}
				slug = defaultSlug
			}
			writeJSONResponse(w, http.StatusOK, CurrentProfilePayload{
				Profile:  slug.String(),
				Registry: registrySlug.String(),
			})
		case http.MethodPost:
			var body CurrentProfilePayload
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			slugRaw := strings.TrimSpace(body.Profile)
			slug, err := gepprofiles.ParseEngineProfileSlug(slugRaw)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, body.Registry)
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			if !IsMockParityProfile(slug.String()) {
				if _, err := profileRegistry.GetEngineProfile(req.Context(), registrySlug, slug); err != nil {
					if errors.Is(err, gepprofiles.ErrProfileNotFound) {
						http.Error(w, "profile not found", http.StatusNotFound)
						return
					}
					http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
					return
				}
			}
			http.SetCookie(w, &http.Cookie{
				Name:     opts.CurrentProfileCookieName,
				Value:    formatCurrentProfileCookieValue(registrySlug, slug),
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Secure:   true,
				HttpOnly: true,
			})
			writeJSONResponse(w, http.StatusOK, CurrentProfilePayload{
				Profile:  slug.String(),
				Registry: registrySlug.String(),
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func formatCurrentProfileCookieValue(registrySlug gepprofiles.RegistrySlug, profileSlug gepprofiles.EngineProfileSlug) string {
	return registrySlug.String() + "/" + profileSlug.String()
}

func resolveDefaultEngineProfileSlug(ctx context.Context, profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug) (gepprofiles.EngineProfileSlug, error) {
	registry, err := profileRegistry.GetRegistry(ctx, registrySlug)
	if err != nil {
		return "", err
	}
	if registry != nil && !registry.DefaultEngineProfileSlug.IsZero() {
		return registry.DefaultEngineProfileSlug, nil
	}
	return gepprofiles.MustEngineProfileSlug("default"), nil
}

func profileExists(ctx context.Context, profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug, slug gepprofiles.EngineProfileSlug) bool {
	if IsMockParityProfile(slug.String()) {
		return true
	}
	_, err := profileRegistry.GetEngineProfile(ctx, registrySlug, slug)
	return err == nil
}
