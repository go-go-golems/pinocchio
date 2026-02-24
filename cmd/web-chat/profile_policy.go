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
			entry := map[string]any{
				"name":   name,
				"config": mw.Config,
			}
			if id := strings.TrimSpace(mw.ID); id != "" {
				entry["id"] = id
			}
			if mw.Enabled != nil {
				entry["enabled"] = *mw.Enabled
			}
			mws = append(mws, entry)
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

type profileDocument = webhttp.ProfileDocument

func registerProfileAPIHandlers(mux *http.ServeMux, resolver *ProfileRequestResolver) {
	if mux == nil || resolver == nil || resolver.profileRegistry == nil {
		return
	}
	middlewareDefinitions, err := newWebChatMiddlewareDefinitionRegistry()
	if err != nil {
		middlewareDefinitions = nil
	}
	webhttp.RegisterProfileAPIHandlers(mux, resolver.profileRegistry, webhttp.ProfileAPIHandlerOptions{
		DefaultRegistrySlug:             resolver.registrySlug,
		EnableCurrentProfileCookieRoute: true,
		CurrentProfileCookieName:        currentProfileCookieName,
		WriteActor:                      profileWriteActor,
		WriteSource:                     profileWriteSource,
		MiddlewareDefinitions:           middlewareDefinitions,
		ExtensionSchemas: []webhttp.ExtensionSchemaDocument{
			{
				Key: "webchat.starter_suggestions@v1",
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"default": []any{},
						},
					},
					"required":             []any{"items"},
					"additionalProperties": false,
				},
			},
		},
	})
}
