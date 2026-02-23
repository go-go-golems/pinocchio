package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/google/uuid"

	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
)

const (
	defaultRegistrySlug      = "default"
	currentProfileCookieName = "chat_profile"

	profileWriteActor  = "web-chat"
	profileWriteSource = "http-api"
)

func buildBootstrapRegistry(defaultSlug string, profileDefs ...*gepprofiles.Profile) (*gepprofiles.ProfileRegistry, error) {
	registrySlug := gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	registry := &gepprofiles.ProfileRegistry{
		Slug:     registrySlug,
		Profiles: map[gepprofiles.ProfileSlug]*gepprofiles.Profile{},
	}

	for _, profile := range profileDefs {
		if profile == nil {
			continue
		}
		clone := profile.Clone()
		if clone == nil {
			continue
		}
		if err := gepprofiles.ValidateProfile(clone); err != nil {
			return nil, err
		}
		registry.Profiles[clone.Slug] = clone
	}

	if strings.TrimSpace(defaultSlug) != "" {
		slug, err := gepprofiles.ParseProfileSlug(defaultSlug)
		if err != nil {
			return nil, err
		}
		registry.DefaultProfileSlug = slug
	}

	if len(registry.Profiles) > 0 {
		if registry.DefaultProfileSlug.IsZero() {
			registry.DefaultProfileSlug = firstProfileSlug(registry.Profiles)
		}
		if _, ok := registry.Profiles[registry.DefaultProfileSlug]; !ok {
			registry.DefaultProfileSlug = firstProfileSlug(registry.Profiles)
		}
	}

	if err := gepprofiles.ValidateRegistry(registry); err != nil {
		return nil, err
	}
	return registry, nil
}

func newInMemoryProfileService(defaultSlug string, profileDefs ...*gepprofiles.Profile) (gepprofiles.Registry, error) {
	registrySlug := gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	registry, err := buildBootstrapRegistry(defaultSlug, profileDefs...)
	if err != nil {
		return nil, err
	}

	store := gepprofiles.NewInMemoryProfileStore()
	if err := store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{Actor: "web-chat", Source: "builtin"}); err != nil {
		return nil, err
	}
	return gepprofiles.NewStoreRegistry(store, registrySlug)
}

func newSQLiteProfileService(
	dsn string,
	dbPath string,
	defaultSlug string,
	profileDefs ...*gepprofiles.Profile,
) (gepprofiles.Registry, func(), error) {
	registrySlug := gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	dsn = strings.TrimSpace(dsn)
	dbPath = strings.TrimSpace(dbPath)

	if dsn == "" {
		if dbPath == "" {
			return nil, nil, fmt.Errorf("profile-registry-dsn or profile-registry-db is required")
		}
		if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, nil, err
			}
		}
		var err error
		dsn, err = gepprofiles.SQLiteProfileDSNForFile(dbPath)
		if err != nil {
			return nil, nil, err
		}
	}

	store, err := gepprofiles.NewSQLiteProfileStore(dsn, registrySlug)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = store.Close() }

	existing, ok, err := store.GetRegistry(context.Background(), registrySlug)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if !ok || existing == nil || len(existing.Profiles) == 0 {
		registry, err := buildBootstrapRegistry(defaultSlug, profileDefs...)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		if err := store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{
			Actor:  profileWriteActor,
			Source: "bootstrap",
		}); err != nil {
			cleanup()
			return nil, nil, err
		}
	}

	svc, err := gepprofiles.NewStoreRegistry(store, registrySlug)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return svc, cleanup, nil
}

func firstProfileSlug(profiles map[gepprofiles.ProfileSlug]*gepprofiles.Profile) gepprofiles.ProfileSlug {
	slugs := make([]gepprofiles.ProfileSlug, 0, len(profiles))
	for slug := range profiles {
		slugs = append(slugs, slug)
	}
	sort.Slice(slugs, func(i, j int) bool { return slugs[i] < slugs[j] })
	if len(slugs) == 0 {
		return ""
	}
	return slugs[0]
}

type ProfileRequestResolver struct {
	profileRegistry gepprofiles.Registry
	registrySlug    gepprofiles.RegistrySlug
}

func newProfileRequestResolver(profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug) *ProfileRequestResolver {
	if registrySlug.IsZero() {
		registrySlug = gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	}
	return &ProfileRequestResolver{profileRegistry: profileRegistry, registrySlug: registrySlug}
}

func (r *ProfileRequestResolver) Resolve(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	if req == nil {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request"}
	}

	switch req.Method {
	case http.MethodGet:
		return r.resolveWS(req)
	case http.MethodPost:
		return r.resolveChat(req)
	default:
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{Status: http.StatusMethodNotAllowed, ClientMsg: "method not allowed"}
	}
}

func (r *ProfileRequestResolver) resolveWS(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
	if convID == "" {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "missing conv_id"}
	}

	_, slug, profile, err := r.resolveProfileSelection(req, "", "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	overrides := runtimeDefaultsFromProfile(profile)
	resolvedRuntime := profileRuntimeSpec(profile)
	profileVersion := uint64(0)
	if profile != nil {
		profileVersion = profile.Metadata.Version
	}

	return webhttp.ResolvedConversationRequest{
		ConvID:          convID,
		RuntimeKey:      slug.String(),
		ProfileVersion:  profileVersion,
		ResolvedRuntime: resolvedRuntime,
		Overrides:       overrides,
	}, nil
}

func (r *ProfileRequestResolver) resolveChat(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	var body webhttp.ChatRequestBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request", Err: err}
	}
	if body.Prompt == "" && body.Text != "" {
		body.Prompt = body.Text
	}

	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}

	pathSlug := profileSlugFromPath(req)
	_, slug, profile, err := r.resolveProfileSelection(req, pathSlug, body.Profile, body.Registry)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}

	overrides, err := mergeRuntimeOverrides(profile, body.Overrides)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedRuntime := profileRuntimeSpec(profile)
	profileVersion := uint64(0)
	if profile != nil {
		profileVersion = profile.Metadata.Version
	}

	return webhttp.ResolvedConversationRequest{
		ConvID:          convID,
		RuntimeKey:      slug.String(),
		ProfileVersion:  profileVersion,
		ResolvedRuntime: resolvedRuntime,
		Overrides:       overrides,
		Prompt:          body.Prompt,
		IdempotencyKey:  strings.TrimSpace(body.IdempotencyKey),
	}, nil
}

func (r *ProfileRequestResolver) resolveProfileSelection(
	req *http.Request,
	pathSlug string,
	bodyProfileRaw string,
	bodyRegistryRaw string,
) (gepprofiles.RegistrySlug, gepprofiles.ProfileSlug, *gepprofiles.Profile, error) {
	if r == nil || r.profileRegistry == nil {
		return "", "", nil, &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile resolver is not configured"}
	}

	registrySlug, err := r.resolveRegistrySlug(req, bodyRegistryRaw)
	if err != nil {
		return "", "", nil, err
	}

	slugRaw := strings.TrimSpace(pathSlug)
	if slugRaw == "" {
		slugRaw = strings.TrimSpace(bodyProfileRaw)
	}
	if slugRaw == "" && req != nil {
		slugRaw = strings.TrimSpace(req.URL.Query().Get("profile"))
	}
	if slugRaw == "" && req != nil {
		slugRaw = strings.TrimSpace(req.URL.Query().Get("runtime"))
	}
	if slugRaw == "" && req != nil {
		if ck, err := req.Cookie(currentProfileCookieName); err == nil && ck != nil {
			slugRaw = strings.TrimSpace(ck.Value)
		}
	}

	ctx := context.Background()
	if strings.TrimSpace(slugRaw) == "" {
		s, err := r.resolveDefaultProfileSlug(ctx, registrySlug)
		if err != nil {
			return "", "", nil, r.toRequestResolutionError(err, "")
		}
		slugRaw = s.String()
	}

	slug, err := gepprofiles.ParseProfileSlug(slugRaw)
	if err != nil {
		return "", "", nil, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid profile: " + slugRaw, Err: err}
	}

	profile, err := r.profileRegistry.GetProfile(ctx, registrySlug, slug)
	if err != nil {
		return "", "", nil, r.toRequestResolutionError(err, slugRaw)
	}
	return registrySlug, slug, profile, nil
}

func (r *ProfileRequestResolver) resolveRegistrySlug(req *http.Request, bodyRegistryRaw string) (gepprofiles.RegistrySlug, error) {
	registryRaw := strings.TrimSpace(bodyRegistryRaw)
	if registryRaw == "" && req != nil {
		registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
	}
	if registryRaw == "" {
		return r.registrySlug, nil
	}
	registrySlug, err := gepprofiles.ParseRegistrySlug(registryRaw)
	if err != nil {
		return "", &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid registry: " + registryRaw, Err: err}
	}
	return registrySlug, nil
}

func (r *ProfileRequestResolver) resolveDefaultProfileSlug(ctx context.Context, registrySlug gepprofiles.RegistrySlug) (gepprofiles.ProfileSlug, error) {
	registry, err := r.profileRegistry.GetRegistry(ctx, registrySlug)
	if err != nil {
		return "", err
	}
	if registry != nil && !registry.DefaultProfileSlug.IsZero() {
		return registry.DefaultProfileSlug, nil
	}
	return gepprofiles.MustProfileSlug("default"), nil
}

func (r *ProfileRequestResolver) toRequestResolutionError(err error, slug string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gepprofiles.ErrProfileNotFound) {
		if strings.TrimSpace(slug) == "" {
			return &webhttp.RequestResolutionError{Status: http.StatusNotFound, ClientMsg: "profile not found"}
		}
		return &webhttp.RequestResolutionError{Status: http.StatusNotFound, ClientMsg: "profile not found: " + slug}
	}
	if errors.Is(err, gepprofiles.ErrRegistryNotFound) {
		return &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile registry is not configured", Err: err}
	}
	return &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile resolution failed", Err: err}
}

func (r *ProfileRequestResolver) profileExists(ctx context.Context, slug gepprofiles.ProfileSlug) bool {
	_, err := r.profileRegistry.GetProfile(ctx, r.registrySlug, slug)
	return err == nil
}

func profileRuntimeSpec(p *gepprofiles.Profile) *gepprofiles.RuntimeSpec {
	if p == nil {
		return nil
	}
	spec := gepprofiles.RuntimeSpec{
		StepSettingsPatch: map[string]any{},
		SystemPrompt:      strings.TrimSpace(p.Runtime.SystemPrompt),
		Middlewares:       append([]gepprofiles.MiddlewareUse(nil), p.Runtime.Middlewares...),
		Tools:             append([]string(nil), p.Runtime.Tools...),
	}
	for k, v := range p.Runtime.StepSettingsPatch {
		spec.StepSettingsPatch[k] = v
	}
	if len(spec.StepSettingsPatch) == 0 {
		spec.StepSettingsPatch = nil
	}
	return &spec
}

func runtimeDefaultsFromProfile(p *gepprofiles.Profile) map[string]any {
	if p == nil {
		return nil
	}
	overrides := map[string]any{}
	if strings.TrimSpace(p.Runtime.SystemPrompt) != "" {
		overrides["system_prompt"] = p.Runtime.SystemPrompt
	}
	if len(p.Runtime.Middlewares) > 0 {
		mws := make([]any, 0, len(p.Runtime.Middlewares))
		for _, mw := range p.Runtime.Middlewares {
			name := strings.TrimSpace(mw.Name)
			if name == "" {
				continue
			}
			mws = append(mws, map[string]any{
				"name":   name,
				"config": mw.Config,
			})
		}
		if len(mws) > 0 {
			overrides["middlewares"] = mws
		}
	}
	if len(p.Runtime.Tools) > 0 {
		tools := make([]any, 0, len(p.Runtime.Tools))
		for _, t := range p.Runtime.Tools {
			name := strings.TrimSpace(t)
			if name == "" {
				continue
			}
			tools = append(tools, name)
		}
		if len(tools) > 0 {
			overrides["tools"] = tools
		}
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

func mergeRuntimeOverrides(profile *gepprofiles.Profile, requestOverrides map[string]any) (map[string]any, error) {
	merged := runtimeDefaultsFromProfile(profile)
	if merged == nil {
		merged = map[string]any{}
	}
	if len(requestOverrides) == 0 {
		if len(merged) == 0 {
			return nil, nil
		}
		return merged, nil
	}

	hasEngineOverride := false
	if _, ok := requestOverrides["system_prompt"]; ok {
		hasEngineOverride = true
	}
	if _, ok := requestOverrides["middlewares"]; ok {
		hasEngineOverride = true
	}
	if _, ok := requestOverrides["tools"]; ok {
		hasEngineOverride = true
	}
	if hasEngineOverride && profile != nil && !profile.Policy.AllowOverrides {
		return nil, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "profile does not allow engine overrides"}
	}

	for k, v := range requestOverrides {
		merged[k] = v
	}
	if len(merged) == 0 {
		return nil, nil
	}
	return merged, nil
}

func profileSlugFromPath(req *http.Request) string {
	if req == nil {
		return ""
	}
	path := req.URL.Path
	if path == "" {
		return ""
	}
	if idx := strings.Index(path, "/chat/"); idx >= 0 {
		rest := path[idx+len("/chat/"):]
		if rest == "" {
			return ""
		}
		if i := strings.Index(rest, "/"); i >= 0 {
			rest = rest[:i]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

type profileListItem struct {
	Slug          string `json:"slug"`
	DisplayName   string `json:"display_name,omitempty"`
	Description   string `json:"description,omitempty"`
	DefaultPrompt string `json:"default_prompt,omitempty"`
	IsDefault     bool   `json:"is_default,omitempty"`
	Version       uint64 `json:"version,omitempty"`
}

type profileDocument struct {
	Registry    string                      `json:"registry"`
	Slug        string                      `json:"slug"`
	DisplayName string                      `json:"display_name,omitempty"`
	Description string                      `json:"description,omitempty"`
	Runtime     gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy      gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata    gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	IsDefault   bool                        `json:"is_default"`
}

type createProfileRequest struct {
	Registry        string                       `json:"registry,omitempty"`
	Slug            string                       `json:"slug,omitempty"`
	Profile         string                       `json:"profile,omitempty"`
	DisplayName     string                       `json:"display_name,omitempty"`
	Description     string                       `json:"description,omitempty"`
	Runtime         *gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy          *gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata        *gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	SetDefault      bool                         `json:"set_default,omitempty"`
	ExpectedVersion uint64                       `json:"expected_version,omitempty"`
}

type patchProfileRequest struct {
	Registry        string                       `json:"registry,omitempty"`
	DisplayName     *string                      `json:"display_name,omitempty"`
	Description     *string                      `json:"description,omitempty"`
	Runtime         *gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy          *gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata        *gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	SetDefault      bool                         `json:"set_default,omitempty"`
	ExpectedVersion uint64                       `json:"expected_version,omitempty"`
}

type setDefaultProfileRequest struct {
	Registry        string `json:"registry,omitempty"`
	ExpectedVersion uint64 `json:"expected_version,omitempty"`
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

func profileDocFromModel(registrySlug gepprofiles.RegistrySlug, registry *gepprofiles.ProfileRegistry, p *gepprofiles.Profile) profileDocument {
	doc := profileDocument{
		Registry: registrySlug.String(),
	}
	if p == nil {
		return doc
	}
	doc.Slug = p.Slug.String()
	doc.DisplayName = p.DisplayName
	doc.Description = p.Description
	doc.Runtime = p.Runtime
	doc.Policy = p.Policy
	doc.Metadata = p.Metadata
	doc.IsDefault = registry != nil && registry.DefaultProfileSlug == p.Slug
	return doc
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

func registerProfileAPIHandlers(mux *http.ServeMux, resolver *ProfileRequestResolver) {
	if mux == nil || resolver == nil || resolver.profileRegistry == nil {
		return
	}

	mux.HandleFunc("/api/chat/profiles", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			registrySlug, err := resolveRegistrySlugForAPI(req, resolver.registrySlug, "")
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			registry, err := resolver.profileRegistry.GetRegistry(context.Background(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			profiles_, err := resolver.profileRegistry.ListProfiles(context.Background(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			items := make([]profileListItem, 0, len(profiles_))
			for _, p := range profiles_ {
				if p == nil {
					continue
				}
				items = append(items, profileListItem{
					Slug:          p.Slug.String(),
					DisplayName:   p.DisplayName,
					Description:   p.Description,
					DefaultPrompt: p.Runtime.SystemPrompt,
					IsDefault:     registry != nil && registry.DefaultProfileSlug == p.Slug,
					Version:       p.Metadata.Version,
				})
			}
			writeJSONResponse(w, http.StatusOK, items)
		case http.MethodPost:
			var body createProfileRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, resolver.registrySlug, body.Registry)
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

			created, err := resolver.profileRegistry.CreateProfile(context.Background(), registrySlug, profile, gepprofiles.WriteOptions{
				ExpectedVersion: body.ExpectedVersion,
				Actor:           profileWriteActor,
				Source:          profileWriteSource,
			})
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			if body.SetDefault {
				if err := resolver.profileRegistry.SetDefaultProfile(context.Background(), registrySlug, created.Slug, gepprofiles.WriteOptions{
					Actor:  profileWriteActor,
					Source: profileWriteSource,
				}); err != nil {
					writeProfileRegistryError(w, err)
					return
				}
			}
			registry, err := resolver.profileRegistry.GetRegistry(context.Background(), registrySlug)
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
				registrySlug, err := resolveRegistrySlugForAPI(req, resolver.registrySlug, "")
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				profile, err := resolver.profileRegistry.GetProfile(context.Background(), registrySlug, slug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				registry, err := resolver.profileRegistry.GetRegistry(context.Background(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
			case http.MethodPatch:
				var body patchProfileRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				registrySlug, err := resolveRegistrySlugForAPI(req, resolver.registrySlug, body.Registry)
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
				}
				profile, err := resolver.profileRegistry.UpdateProfile(context.Background(), registrySlug, slug, patch, gepprofiles.WriteOptions{
					ExpectedVersion: body.ExpectedVersion,
					Actor:           profileWriteActor,
					Source:          profileWriteSource,
				})
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				if body.SetDefault {
					if err := resolver.profileRegistry.SetDefaultProfile(context.Background(), registrySlug, slug, gepprofiles.WriteOptions{
						Actor:  profileWriteActor,
						Source: profileWriteSource,
					}); err != nil {
						writeProfileRegistryError(w, err)
						return
					}
				}
				registry, err := resolver.profileRegistry.GetRegistry(context.Background(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
			case http.MethodDelete:
				registrySlug, err := resolveRegistrySlugForAPI(req, resolver.registrySlug, "")
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				expectedVersion, err := parseExpectedVersion(req.URL.Query().Get("expected_version"))
				if err != nil {
					http.Error(w, "invalid expected_version", http.StatusBadRequest)
					return
				}
				if err := resolver.profileRegistry.DeleteProfile(context.Background(), registrySlug, slug, gepprofiles.WriteOptions{
					ExpectedVersion: expectedVersion,
					Actor:           profileWriteActor,
					Source:          profileWriteSource,
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
			var body setDefaultProfileRequest
			if req.Body != nil {
				_ = json.NewDecoder(req.Body).Decode(&body)
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, resolver.registrySlug, body.Registry)
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
			if err := resolver.profileRegistry.SetDefaultProfile(context.Background(), registrySlug, slug, gepprofiles.WriteOptions{
				ExpectedVersion: expectedVersion,
				Actor:           profileWriteActor,
				Source:          profileWriteSource,
			}); err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			profile, err := resolver.profileRegistry.GetProfile(context.Background(), registrySlug, slug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			registry, err := resolver.profileRegistry.GetRegistry(context.Background(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
		default:
			http.NotFound(w, req)
		}
	})

	mux.HandleFunc("/api/chat/profile", func(w http.ResponseWriter, req *http.Request) {
		type profilePayload struct {
			Slug    string `json:"slug"`
			Profile string `json:"profile"`
		}
		switch req.Method {
		case http.MethodGet:
			slug := gepprofiles.ProfileSlug("")
			if ck, err := req.Cookie(currentProfileCookieName); err == nil && ck != nil {
				if parsed, err := gepprofiles.ParseProfileSlug(strings.TrimSpace(ck.Value)); err == nil && resolver.profileExists(context.Background(), parsed) {
					slug = parsed
				}
			}
			if slug.IsZero() {
				defaultSlug, err := resolver.resolveDefaultProfileSlug(context.Background(), resolver.registrySlug)
				if err != nil {
					http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
					return
				}
				slug = defaultSlug
			}
			writeJSONResponse(w, http.StatusOK, profilePayload{Slug: slug.String()})
		case http.MethodPost:
			var body profilePayload
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
			if _, err := resolver.profileRegistry.GetProfile(context.Background(), resolver.registrySlug, slug); err != nil {
				if errors.Is(err, gepprofiles.ErrProfileNotFound) {
					http.Error(w, "profile not found", http.StatusNotFound)
					return
				}
				http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     currentProfileCookieName,
				Value:    slug.String(),
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Secure:   true,
				HttpOnly: true,
			})
			writeJSONResponse(w, http.StatusOK, profilePayload{Slug: slug.String()})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
