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

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/google/uuid"

	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
)

const (
	defaultRegistrySlug      = "default"
	currentProfileCookieName = "chat_profile"
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
			Actor:  "web-chat",
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
	profileRegistry       gepprofiles.Registry
	defaultRegistrySlug   gepprofiles.RegistrySlug
	baseInferenceSettings *aisettings.InferenceSettings
}

func newProfileRequestResolver(profileRegistry gepprofiles.Registry, defaultRegistry gepprofiles.RegistrySlug, baseInferenceSettings *aisettings.InferenceSettings) *ProfileRequestResolver {
	if defaultRegistry.IsZero() {
		defaultRegistry = gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	}
	return &ProfileRequestResolver{
		profileRegistry:       profileRegistry,
		defaultRegistrySlug:   defaultRegistry,
		baseInferenceSettings: cloneResolvedInferenceSettings(baseInferenceSettings),
	}
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

	if err := rejectLegacyProfileSelectors(req, "", ""); err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	profileSlug, err := r.resolveProfileSelection(req, "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	registrySlug, err := r.resolveRegistrySelection(req, "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedProfile, err := r.resolveEffectiveProfile(context.Background(), registrySlug, profileSlug)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedRuntime := resolvedProfile.EffectiveRuntime

	return webhttp.ResolvedConversationRequest{
		ConvID:                    convID,
		RuntimeKey:                runtimeKeyFromResolvedProfile(resolvedProfile),
		RuntimeFingerprint:        resolvedProfile.RuntimeFingerprint,
		ProfileVersion:            profileVersionFromResolvedMetadata(resolvedProfile.Metadata),
		ResolvedInferenceSettings: cloneResolvedInferenceSettings(r.baseInferenceSettings),
		ResolvedRuntime:           &resolvedRuntime,
		ProfileMetadata:           copyMetadataMap(resolvedProfile.Metadata),
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
	if err := rejectLegacyProfileSelectors(req, body.LegacyRuntimeKey, body.LegacyRegistry); err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}

	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}

	pathSlug := profileSlugFromPath(req)
	profileSlug, err := r.resolveProfileSelection(req, pathSlug, body.Profile)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	registrySlug, err := r.resolveRegistrySelection(req, body.Registry)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedProfile, err := r.resolveEffectiveProfile(context.Background(), registrySlug, profileSlug)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedRuntime := resolvedProfile.EffectiveRuntime

	return webhttp.ResolvedConversationRequest{
		ConvID:                    convID,
		RuntimeKey:                runtimeKeyFromResolvedProfile(resolvedProfile),
		RuntimeFingerprint:        resolvedProfile.RuntimeFingerprint,
		ProfileVersion:            profileVersionFromResolvedMetadata(resolvedProfile.Metadata),
		ResolvedInferenceSettings: cloneResolvedInferenceSettings(r.baseInferenceSettings),
		ResolvedRuntime:           &resolvedRuntime,
		ProfileMetadata:           copyMetadataMap(resolvedProfile.Metadata),
		Prompt:                    body.Prompt,
		IdempotencyKey:            strings.TrimSpace(body.IdempotencyKey),
	}, nil
}

func (r *ProfileRequestResolver) resolveProfileSelection(
	req *http.Request,
	pathSlug string,
	bodyProfileRaw string,
) (gepprofiles.ProfileSlug, error) {
	if r == nil || r.profileRegistry == nil {
		return "", &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile resolver is not configured"}
	}

	slugRaw := strings.TrimSpace(pathSlug)
	if slugRaw == "" {
		slugRaw = strings.TrimSpace(bodyProfileRaw)
	}
	if slugRaw == "" && req != nil {
		slugRaw = strings.TrimSpace(req.URL.Query().Get("profile"))
	}
	if slugRaw == "" && req != nil {
		if ck, err := req.Cookie(currentProfileCookieName); err == nil && ck != nil {
			if cookieProfile, ok := r.resolveProfileSlugFromCookie(req.Context(), strings.TrimSpace(ck.Value)); ok {
				slugRaw = cookieProfile.String()
			}
		}
	}

	if strings.TrimSpace(slugRaw) == "" {
		return "", nil
	}

	slug, err := gepprofiles.ParseProfileSlug(slugRaw)
	if err != nil {
		return "", &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid profile: " + slugRaw, Err: err}
	}
	return slug, nil
}

func (r *ProfileRequestResolver) resolveEffectiveProfile(
	ctx context.Context,
	registrySlug gepprofiles.RegistrySlug,
	profileSlug gepprofiles.ProfileSlug,
) (*gepprofiles.ResolvedProfile, error) {
	in := gepprofiles.ResolveInput{
		RegistrySlug: registrySlug,
		ProfileSlug:  profileSlug,
	}
	resolved, err := r.profileRegistry.ResolveEffectiveProfile(ctx, in)
	if err != nil {
		return nil, r.toRequestResolutionError(err, profileSlug.String())
	}
	return resolved, nil
}

func runtimeKeyFromResolvedProfile(resolved *gepprofiles.ResolvedProfile) string {
	if resolved == nil {
		return ""
	}
	if slug := strings.TrimSpace(resolved.ProfileSlug.String()); slug != "" {
		return slug
	}
	return "default"
}

func cloneResolvedInferenceSettings(in *aisettings.InferenceSettings) *aisettings.InferenceSettings {
	if in == nil {
		return nil
	}
	return in.Clone()
}

func (r *ProfileRequestResolver) resolveRegistrySelection(req *http.Request, bodyRegistryRaw string) (gepprofiles.RegistrySlug, error) {
	if r == nil || r.profileRegistry == nil {
		return "", &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile resolver is not configured"}
	}

	registryRaw := strings.TrimSpace(bodyRegistryRaw)
	if registryRaw == "" && req != nil {
		registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
	}
	if registryRaw == "" && req != nil {
		if ck, err := req.Cookie(currentProfileCookieName); err == nil && ck != nil {
			if cookieRegistry, _, ok := parseCurrentProfileCookieValue(strings.TrimSpace(ck.Value)); ok {
				registryRaw = cookieRegistry.String()
			}
		}
	}
	if registryRaw == "" {
		return r.defaultRegistrySlug, nil
	}
	registrySlug, err := gepprofiles.ParseRegistrySlug(registryRaw)
	if err != nil {
		return "", &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid registry: " + registryRaw, Err: err}
	}
	return registrySlug, nil
}

func (r *ProfileRequestResolver) resolveProfileSlugFromCookie(ctx context.Context, raw string) (gepprofiles.ProfileSlug, bool) {
	if r == nil || r.profileRegistry == nil {
		return "", false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, cookieProfile, ok := parseCurrentProfileCookieValue(raw); ok {
		return cookieProfile, true
	}

	legacyProfile, err := gepprofiles.ParseProfileSlug(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	if _, err := r.profileRegistry.GetProfile(ctx, r.defaultRegistrySlug, legacyProfile); err != nil {
		return "", false
	}
	return legacyProfile, true
}

func rejectLegacyProfileSelectors(req *http.Request, legacyRuntimeKey string, legacyRegistry string) error {
	if req != nil {
		query := req.URL.Query()
		if _, ok := query["runtime_key"]; ok {
			return &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "unsupported legacy selector: runtime_key"}
		}
		if _, ok := query["registry_slug"]; ok {
			return &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "unsupported legacy selector: registry_slug"}
		}
	}
	if strings.TrimSpace(legacyRuntimeKey) != "" {
		return &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "unsupported legacy selector: runtime_key"}
	}
	if strings.TrimSpace(legacyRegistry) != "" {
		return &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "unsupported legacy selector: registry_slug"}
	}
	return nil
}

func parseCurrentProfileCookieValue(raw string) (gepprofiles.RegistrySlug, gepprofiles.ProfileSlug, bool) {
	parts := strings.SplitN(strings.TrimSpace(raw), "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	registrySlug, err := gepprofiles.ParseRegistrySlug(parts[0])
	if err != nil {
		return "", "", false
	}
	profileSlug, err := gepprofiles.ParseProfileSlug(parts[1])
	if err != nil {
		return "", "", false
	}
	return registrySlug, profileSlug, true
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

func copyMetadataMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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
		DefaultRegistrySlug:             resolver.defaultRegistrySlug,
		EnableCurrentProfileCookieRoute: true,
		CurrentProfileCookieName:        currentProfileCookieName,
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
