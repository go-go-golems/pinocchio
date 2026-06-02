package profiles

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
)

func registerProfileHandlers(mux *http.ServeMux, profileRegistry gepprofiles.Registry, opts APIOptions) {
	mux.HandleFunc("/api/chat/profiles", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		registryRaw := ""
		if req != nil {
			registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
		}
		items := make([]ProfileListItem, 0)
		if registryRaw != "" {
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			profiles_, err := profileRegistry.ListEngineProfiles(req.Context(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			items = append(items, profileListItemsFromRegistry(registrySlug, registry, profiles_)...)
			if registrySlug == opts.DefaultRegistrySlug {
				items = append(items, mockParityProfileListItem(registrySlug))
			}
		} else {
			registries, err := profileRegistry.ListRegistries(req.Context())
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			for _, summary := range registries {
				registrySlug := summary.Slug
				registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				profiles_, err := profileRegistry.ListEngineProfiles(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				items = append(items, profileListItemsFromRegistry(registrySlug, registry, profiles_)...)
			}
			items = append(items, mockParityProfileListItem(opts.DefaultRegistrySlug))
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].Slug < items[j].Slug
		})
		writeJSONResponse(w, http.StatusOK, items)
	})

	mux.HandleFunc("/api/chat/profiles/", func(w http.ResponseWriter, req *http.Request) {
		slugRaw, action, ok := parseProfilePath(req.URL.Path)
		if !ok {
			http.NotFound(w, req)
			return
		}
		slug, err := gepprofiles.ParseEngineProfileSlug(slugRaw)
		if err != nil {
			http.Error(w, "invalid profile slug", http.StatusBadRequest)
			return
		}

		switch action {
		case "":
			if req.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if IsMockParityProfile(slug.String()) {
				writeJSONResponse(w, http.StatusOK, mockParityProfileDocument(opts.DefaultRegistrySlug))
				return
			}
			registryRaw := ""
			if req != nil {
				registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
			}
			if registryRaw != "" {
				registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				profile, err := profileRegistry.GetEngineProfile(req.Context(), registrySlug, slug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
				return
			}

			registries, err := profileRegistry.ListRegistries(req.Context())
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			for _, summary := range registries {
				registrySlug := summary.Slug
				profile, err := profileRegistry.GetEngineProfile(req.Context(), registrySlug, slug)
				if err != nil {
					if errors.Is(err, gepprofiles.ErrProfileNotFound) {
						continue
					}
					writeProfileRegistryError(w, err)
					return
				}
				registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
				return
			}
			writeProfileRegistryError(w, gepprofiles.ErrProfileNotFound)
		case "default":
			if req.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			profile, err := profileRegistry.GetEngineProfile(req.Context(), registrySlug, slug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
		default:
			http.NotFound(w, req)
		}
	})
}

func parseProfilePath(path string) (string, string, bool) {
	const prefix = "/api/chat/profiles/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) > 2 {
		return "", "", false
	}
	slug := strings.TrimSpace(parts[0])
	if slug == "" {
		return "", "", false
	}
	action := ""
	if len(parts) == 2 {
		action = strings.TrimSpace(parts[1])
	}
	return slug, action, true
}

func resolveRegistrySlugForAPI(req *http.Request, defaultSlug gepprofiles.RegistrySlug, bodyRegistryRaw string) (gepprofiles.RegistrySlug, error) {
	registryRaw := strings.TrimSpace(bodyRegistryRaw)
	if registryRaw == "" && req != nil {
		registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
	}
	if registryRaw == "" {
		return defaultSlug, nil
	}
	registrySlug, err := gepprofiles.ParseRegistrySlug(registryRaw)
	if err != nil {
		return "", err
	}
	return registrySlug, nil
}
