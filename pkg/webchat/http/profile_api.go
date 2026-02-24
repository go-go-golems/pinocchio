package webhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
)

const (
	defaultProfileAPIRegistrySlug = "default"
	defaultCurrentProfileCookie   = "chat_profile"
	defaultProfileWriteActor      = "web-chat"
	defaultProfileWriteSource     = "http-api"
)

type ProfileListItem struct {
	Slug          string         `json:"slug"`
	DisplayName   string         `json:"display_name,omitempty"`
	Description   string         `json:"description,omitempty"`
	DefaultPrompt string         `json:"default_prompt,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
	IsDefault     bool           `json:"is_default,omitempty"`
	Version       uint64         `json:"version,omitempty"`
}

type ProfileDocument struct {
	Registry    string                      `json:"registry"`
	Slug        string                      `json:"slug"`
	DisplayName string                      `json:"display_name,omitempty"`
	Description string                      `json:"description,omitempty"`
	Runtime     gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy      gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata    gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	Extensions  map[string]any              `json:"extensions,omitempty"`
	IsDefault   bool                        `json:"is_default"`
}

type CreateProfileRequest struct {
	Registry        string                       `json:"registry,omitempty"`
	Slug            string                       `json:"slug,omitempty"`
	Profile         string                       `json:"profile,omitempty"`
	DisplayName     string                       `json:"display_name,omitempty"`
	Description     string                       `json:"description,omitempty"`
	Runtime         *gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy          *gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata        *gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	Extensions      map[string]any               `json:"extensions,omitempty"`
	SetDefault      bool                         `json:"set_default,omitempty"`
	ExpectedVersion uint64                       `json:"expected_version,omitempty"`
}

type PatchProfileRequest struct {
	Registry        string                       `json:"registry,omitempty"`
	DisplayName     *string                      `json:"display_name,omitempty"`
	Description     *string                      `json:"description,omitempty"`
	Runtime         *gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy          *gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata        *gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	Extensions      *map[string]any              `json:"extensions,omitempty"`
	SetDefault      bool                         `json:"set_default,omitempty"`
	ExpectedVersion uint64                       `json:"expected_version,omitempty"`
}

type SetDefaultProfileRequest struct {
	Registry        string `json:"registry,omitempty"`
	ExpectedVersion uint64 `json:"expected_version,omitempty"`
}

type CurrentProfilePayload struct {
	Slug    string `json:"slug"`
	Profile string `json:"profile,omitempty"`
}

type ProfileAPIHandlerOptions struct {
	DefaultRegistrySlug             gepprofiles.RegistrySlug
	EnableCurrentProfileCookieRoute bool
	CurrentProfileCookieName        string
	WriteActor                      string
	WriteSource                     string
}

func (o *ProfileAPIHandlerOptions) normalize() {
	if o.DefaultRegistrySlug.IsZero() {
		o.DefaultRegistrySlug = gepprofiles.MustRegistrySlug(defaultProfileAPIRegistrySlug)
	}
	if strings.TrimSpace(o.CurrentProfileCookieName) == "" {
		o.CurrentProfileCookieName = defaultCurrentProfileCookie
	}
	if strings.TrimSpace(o.WriteActor) == "" {
		o.WriteActor = defaultProfileWriteActor
	}
	if strings.TrimSpace(o.WriteSource) == "" {
		o.WriteSource = defaultProfileWriteSource
	}
}

func RegisterProfileAPIHandlers(mux *http.ServeMux, profileRegistry gepprofiles.Registry, opts ProfileAPIHandlerOptions) {
	if mux == nil || profileRegistry == nil {
		return
	}
	opts.normalize()

	mux.HandleFunc("/api/chat/profiles", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
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
			profiles_, err := profileRegistry.ListProfiles(req.Context(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			sort.Slice(profiles_, func(i, j int) bool {
				if profiles_[i] == nil {
					return false
				}
				if profiles_[j] == nil {
					return true
				}
				return profiles_[i].Slug < profiles_[j].Slug
			})
			items := make([]ProfileListItem, 0, len(profiles_))
			for _, p := range profiles_ {
				if p == nil {
					continue
				}
				items = append(items, ProfileListItem{
					Slug:          p.Slug.String(),
					DisplayName:   p.DisplayName,
					Description:   p.Description,
					DefaultPrompt: p.Runtime.SystemPrompt,
					Extensions:    cloneExtensionMap(p.Extensions),
					IsDefault:     registry != nil && registry.DefaultProfileSlug == p.Slug,
					Version:       p.Metadata.Version,
				})
			}
			writeJSONResponse(w, http.StatusOK, items)
		case http.MethodPost:
			var body CreateProfileRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, body.Registry)
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			slugRaw := strings.TrimSpace(body.Slug)
			if slugRaw == "" {
				slugRaw = strings.TrimSpace(body.Profile)
			}
			slug, err := gepprofiles.ParseProfileSlug(slugRaw)
			if err != nil {
				http.Error(w, "invalid profile slug", http.StatusBadRequest)
				return
			}
			profile := &gepprofiles.Profile{
				Slug:        slug,
				DisplayName: strings.TrimSpace(body.DisplayName),
				Description: strings.TrimSpace(body.Description),
			}
			if body.Runtime != nil {
				profile.Runtime = *body.Runtime
			}
			if body.Policy != nil {
				profile.Policy = *body.Policy
			}
			if body.Metadata != nil {
				profile.Metadata = *body.Metadata
			}
			if len(body.Extensions) > 0 {
				profile.Extensions = cloneExtensionMap(body.Extensions)
			}

			created, err := profileRegistry.CreateProfile(req.Context(), registrySlug, profile, gepprofiles.WriteOptions{
				ExpectedVersion: body.ExpectedVersion,
				Actor:           opts.WriteActor,
				Source:          opts.WriteSource,
			})
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			if body.SetDefault {
				if err := profileRegistry.SetDefaultProfile(req.Context(), registrySlug, created.Slug, gepprofiles.WriteOptions{
					Actor:  opts.WriteActor,
					Source: opts.WriteSource,
				}); err != nil {
					writeProfileRegistryError(w, err)
					return
				}
			}
			registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			writeJSONResponse(w, http.StatusCreated, profileDocFromModel(registrySlug, registry, created))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/chat/profiles/", func(w http.ResponseWriter, req *http.Request) {
		slugRaw, action, ok := parseProfilePath(req.URL.Path)
		if !ok {
			http.NotFound(w, req)
			return
		}
		slug, err := gepprofiles.ParseProfileSlug(slugRaw)
		if err != nil {
			http.Error(w, "invalid profile slug", http.StatusBadRequest)
			return
		}

		switch action {
		case "":
			switch req.Method {
			case http.MethodGet:
				registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				profile, err := profileRegistry.GetProfile(req.Context(), registrySlug, slug)
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
			case http.MethodPatch:
				var body PatchProfileRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, body.Registry)
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				patch := gepprofiles.ProfilePatch{
					DisplayName: body.DisplayName,
					Description: body.Description,
					Runtime:     body.Runtime,
					Policy:      body.Policy,
					Metadata:    body.Metadata,
					Extensions:  body.Extensions,
				}
				profile, err := profileRegistry.UpdateProfile(req.Context(), registrySlug, slug, patch, gepprofiles.WriteOptions{
					ExpectedVersion: body.ExpectedVersion,
					Actor:           opts.WriteActor,
					Source:          opts.WriteSource,
				})
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				if body.SetDefault {
					if err := profileRegistry.SetDefaultProfile(req.Context(), registrySlug, slug, gepprofiles.WriteOptions{
						Actor:  opts.WriteActor,
						Source: opts.WriteSource,
					}); err != nil {
						writeProfileRegistryError(w, err)
						return
					}
				}
				registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
			case http.MethodDelete:
				registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				expectedVersion, err := parseExpectedVersion(req.URL.Query().Get("expected_version"))
				if err != nil {
					http.Error(w, "invalid expected_version", http.StatusBadRequest)
					return
				}
				if err := profileRegistry.DeleteProfile(req.Context(), registrySlug, slug, gepprofiles.WriteOptions{
					ExpectedVersion: expectedVersion,
					Actor:           opts.WriteActor,
					Source:          opts.WriteSource,
				}); err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		case "default":
			if req.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var body SetDefaultProfileRequest
			if req.Body != nil {
				_ = json.NewDecoder(req.Body).Decode(&body)
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, body.Registry)
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			expectedVersion := body.ExpectedVersion
			if expectedVersion == 0 {
				expectedVersion, err = parseExpectedVersion(req.URL.Query().Get("expected_version"))
				if err != nil {
					http.Error(w, "invalid expected_version", http.StatusBadRequest)
					return
				}
			}
			if err := profileRegistry.SetDefaultProfile(req.Context(), registrySlug, slug, gepprofiles.WriteOptions{
				ExpectedVersion: expectedVersion,
				Actor:           opts.WriteActor,
				Source:          opts.WriteSource,
			}); err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			profile, err := profileRegistry.GetProfile(req.Context(), registrySlug, slug)
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

	if !opts.EnableCurrentProfileCookieRoute {
		return
	}

	mux.HandleFunc("/api/chat/profile", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			slug := gepprofiles.ProfileSlug("")
			if ck, err := req.Cookie(opts.CurrentProfileCookieName); err == nil && ck != nil {
				if parsed, err := gepprofiles.ParseProfileSlug(strings.TrimSpace(ck.Value)); err == nil && profileExists(req.Context(), profileRegistry, opts.DefaultRegistrySlug, parsed) {
					slug = parsed
				}
			}
			if slug.IsZero() {
				defaultSlug, err := resolveDefaultProfileSlug(req.Context(), profileRegistry, opts.DefaultRegistrySlug)
				if err != nil {
					http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
					return
				}
				slug = defaultSlug
			}
			writeJSONResponse(w, http.StatusOK, CurrentProfilePayload{Slug: slug.String()})
		case http.MethodPost:
			var body CurrentProfilePayload
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			slugRaw := strings.TrimSpace(body.Slug)
			if slugRaw == "" {
				slugRaw = strings.TrimSpace(body.Profile)
			}
			slug, err := gepprofiles.ParseProfileSlug(slugRaw)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if _, err := profileRegistry.GetProfile(req.Context(), opts.DefaultRegistrySlug, slug); err != nil {
				if errors.Is(err, gepprofiles.ErrProfileNotFound) {
					http.Error(w, "profile not found", http.StatusNotFound)
					return
				}
				http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     opts.CurrentProfileCookieName,
				Value:    slug.String(),
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Secure:   true,
				HttpOnly: true,
			})
			writeJSONResponse(w, http.StatusOK, CurrentProfilePayload{Slug: slug.String()})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

func parseExpectedVersion(raw string) (uint64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	v, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expected_version: %w", err)
	}
	return v, nil
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

func profileDocFromModel(registrySlug gepprofiles.RegistrySlug, registry *gepprofiles.ProfileRegistry, p *gepprofiles.Profile) ProfileDocument {
	doc := ProfileDocument{Registry: registrySlug.String()}
	if p == nil {
		return doc
	}
	doc.Slug = p.Slug.String()
	doc.DisplayName = p.DisplayName
	doc.Description = p.Description
	doc.Runtime = p.Runtime
	doc.Policy = p.Policy
	doc.Metadata = p.Metadata
	doc.Extensions = cloneExtensionMap(p.Extensions)
	doc.IsDefault = registry != nil && registry.DefaultProfileSlug == p.Slug
	return doc
}

func cloneExtensionMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

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
	case errors.Is(err, gepprofiles.ErrPolicyViolation):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, gepprofiles.ErrVersionConflict):
		http.Error(w, err.Error(), http.StatusConflict)
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

func resolveDefaultProfileSlug(ctx context.Context, profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug) (gepprofiles.ProfileSlug, error) {
	registry, err := profileRegistry.GetRegistry(ctx, registrySlug)
	if err != nil {
		return "", err
	}
	if registry != nil && !registry.DefaultProfileSlug.IsZero() {
		return registry.DefaultProfileSlug, nil
	}
	return gepprofiles.MustProfileSlug("default"), nil
}

func profileExists(ctx context.Context, profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug, slug gepprofiles.ProfileSlug) bool {
	_, err := profileRegistry.GetProfile(ctx, registrySlug, slug)
	return err == nil
}
