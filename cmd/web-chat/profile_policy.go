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

	registrySlug, profileSlug, err := r.resolveProfileSelection(req, "", "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedProfile, err := r.resolveEffectiveProfile(context.Background(), registrySlug, profileSlug, nil)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedRuntime := resolvedProfile.EffectiveRuntime

	return webhttp.ResolvedConversationRequest{
		ConvID:          convID,
		RuntimeKey:      resolvedProfile.RuntimeKey.String(),
		ProfileVersion:  profileVersionFromResolvedMetadata(resolvedProfile.Metadata),
		ResolvedRuntime: &resolvedRuntime,
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
	registrySlug, profileSlug, err := r.resolveProfileSelection(req, pathSlug, body.Profile, body.Registry)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedProfile, err := r.resolveEffectiveProfile(context.Background(), registrySlug, profileSlug, body.Overrides)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedRuntime := resolvedProfile.EffectiveRuntime

	return webhttp.ResolvedConversationRequest{
		ConvID:          convID,
		RuntimeKey:      resolvedProfile.RuntimeKey.String(),
		ProfileVersion:  profileVersionFromResolvedMetadata(resolvedProfile.Metadata),
		ResolvedRuntime: &resolvedRuntime,
		Prompt:          body.Prompt,
		IdempotencyKey:  strings.TrimSpace(body.IdempotencyKey),
	}, nil
}

func (r *ProfileRequestResolver) resolveProfileSelection(
	req *http.Request,
	pathSlug string,
	bodyProfileRaw string,
	bodyRegistryRaw string,
) (gepprofiles.RegistrySlug, gepprofiles.ProfileSlug, error) {
	if r == nil || r.profileRegistry == nil {
		return "", "", &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile resolver is not configured"}
	}

	registrySlug, err := r.resolveRegistrySlug(req, bodyRegistryRaw)
	if err != nil {
		return "", "", err
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

	if strings.TrimSpace(slugRaw) == "" {
		return registrySlug, "", nil
	}

	slug, err := gepprofiles.ParseProfileSlug(slugRaw)
	if err != nil {
		return "", "", &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid profile: " + slugRaw, Err: err}
	}
	return registrySlug, slug, nil
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

func (r *ProfileRequestResolver) resolveEffectiveProfile(
	ctx context.Context,
	registrySlug gepprofiles.RegistrySlug,
	profileSlug gepprofiles.ProfileSlug,
	requestOverrides map[string]any,
) (*gepprofiles.ResolvedProfile, error) {
	in := gepprofiles.ResolveInput{
		RegistrySlug:     registrySlug,
		ProfileSlug:      profileSlug,
		RequestOverrides: requestOverrides,
	}
	if !profileSlug.IsZero() {
		if runtimeKey, err := gepprofiles.ParseRuntimeKey(profileSlug.String()); err == nil {
			in.RuntimeKeyFallback = runtimeKey
		}
	}
	resolved, err := r.profileRegistry.ResolveEffectiveProfile(ctx, in)
	if err != nil {
		return nil, r.toRequestResolutionError(err, profileSlug.String())
	}
	return resolved, nil
}

func profileVersionFromResolvedMetadata(metadata map[string]any) uint64 {
	raw := metadata["profile.version"]
	switch v := raw.(type) {
	case uint64:
		return v
	case uint32:
		return uint64(v)
	case uint:
		return uint64(v)
	case int64:
		if v >= 0 {
			return uint64(v)
		}
	case int:
		if v >= 0 {
			return uint64(v)
		}
	case float64:
		if v >= 0 {
			return uint64(v)
		}
	}
	return 0
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
		return &webhttp.RequestResolutionError{Status: http.StatusNotFound, ClientMsg: "registry not found", Err: err}
	}
	var validationErr *gepprofiles.ValidationError
	if errors.As(err, &validationErr) {
		return &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: validationErr.Error(), Err: err}
	}
	var policyErr *gepprofiles.PolicyViolationError
	if errors.As(err, &policyErr) {
		return &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: policyErr.Error(), Err: err}
	}
	return &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile resolution failed", Err: err}
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
