package profiles

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

const DefaultRegistrySlug = "default"

const defaultRegistrySlug = "default"

// RequestResolver resolves profile selection and builds conversation plans.
type RequestResolver struct {
	profileRegistry       gepprofiles.Registry
	defaultRegistrySlug   gepprofiles.RegistrySlug
	baseInferenceSettings *aisettings.InferenceSettings
}

// NewRequestResolver creates a new profile request resolver.
func NewRequestResolver(profileRegistry gepprofiles.Registry, defaultRegistry gepprofiles.RegistrySlug, baseInferenceSettings *aisettings.InferenceSettings) *RequestResolver {
	if defaultRegistry.IsZero() {
		defaultRegistry = gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	}
	return &RequestResolver{
		profileRegistry:       profileRegistry,
		defaultRegistrySlug:   defaultRegistry,
		baseInferenceSettings: CloneResolvedInferenceSettings(baseInferenceSettings),
	}
}

// Registry returns the underlying profile registry.
func (r *RequestResolver) Registry() gepprofiles.Registry {
	if r == nil {
		return nil
	}
	return r.profileRegistry
}

// DefaultRegistrySlug returns the default registry slug.
func (r *RequestResolver) DefaultRegistrySlug() gepprofiles.RegistrySlug {
	if r == nil {
		return gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	}
	return r.defaultRegistrySlug
}

// ResolveProfileSelection determines the effective profile slug from request inputs.
func (r *RequestResolver) ResolveProfileSelection(reqCtx context.Context, pathSlug, bodyProfile, queryProfile, cookieValue string) (gepprofiles.EngineProfileSlug, error) {
	slugRaw := strings.TrimSpace(pathSlug)
	if slugRaw == "" {
		slugRaw = strings.TrimSpace(bodyProfile)
	}
	if slugRaw == "" {
		slugRaw = strings.TrimSpace(queryProfile)
	}
	if slugRaw == "" && cookieValue != "" {
		if cookieProfile, ok := r.resolveProfileSlugFromCookie(reqCtx, strings.TrimSpace(cookieValue)); ok {
			slugRaw = cookieProfile.String()
		}
	}

	if strings.TrimSpace(slugRaw) == "" {
		return "", nil
	}
	if r == nil || r.profileRegistry == nil {
		return "", &RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "profile selection requires configured profile registries",
		}
	}

	slug, err := gepprofiles.ParseEngineProfileSlug(slugRaw)
	if err != nil {
		return "", &RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid profile: " + slugRaw, Err: err}
	}
	return slug, nil
}

// ResolveRegistrySelection determines the effective registry slug.
func (r *RequestResolver) ResolveRegistrySelection(bodyRegistryRaw, queryRegistryRaw, cookieValue string) (gepprofiles.RegistrySlug, error) {
	registryRaw := strings.TrimSpace(bodyRegistryRaw)
	if registryRaw == "" {
		registryRaw = strings.TrimSpace(queryRegistryRaw)
	}
	if registryRaw == "" && r != nil && r.profileRegistry != nil && cookieValue != "" {
		if cookieRegistry, _, ok := parseCurrentProfileCookieValue(strings.TrimSpace(cookieValue)); ok {
			registryRaw = cookieRegistry.String()
		}
	}
	if registryRaw == "" {
		if r == nil {
			return "", nil
		}
		return r.defaultRegistrySlug, nil
	}
	if r == nil || r.profileRegistry == nil {
		return "", &RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "registry selection requires configured profile registries",
		}
	}
	registrySlug, err := gepprofiles.ParseRegistrySlug(registryRaw)
	if err != nil {
		return "", &RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid registry: " + registryRaw, Err: err}
	}
	return registrySlug, nil
}

// ResolveEffectiveProfile resolves the full effective profile from registry + slug.
func (r *RequestResolver) ResolveEffectiveProfile(ctx context.Context, registrySlug gepprofiles.RegistrySlug, profileSlug gepprofiles.EngineProfileSlug) (*gepprofiles.ResolvedEngineProfile, error) {
	if r == nil || r.profileRegistry == nil {
		return nil, nil
	}
	in := gepprofiles.ResolveInput{
		RegistrySlug:      registrySlug,
		EngineProfileSlug: profileSlug,
	}
	resolved, err := r.profileRegistry.ResolveEngineProfile(ctx, in)
	if err != nil {
		return nil, r.toRequestResolutionError(err, profileSlug.String())
	}
	return resolved, nil
}

// BuildConversationPlan builds a conversation plan from a resolved profile.
func (r *RequestResolver) BuildConversationPlan(ctx context.Context, convID, prompt, idempotencyKey string, resolvedProfile *gepprofiles.ResolvedEngineProfile) (*ConversationPlan, error) {
	resolvedPlan, err := r.resolveRuntimePlan(ctx, resolvedProfile)
	if err != nil {
		return nil, err
	}
	runtimeKey := runtimeKeyFromResolvedProfile(resolvedProfile)

	rt := &ResolvedRuntime{
		SystemPrompt:      "",
		Middlewares:       nil,
		ToolNames:         nil,
		RuntimeKey:        runtimeKey,
		ProfileVersion:    0,
		InferenceSettings: nil,
		ProfileMetadata:   nil,
	}
	if resolvedPlan != nil {
		rt.ProfileVersion = resolvedPlan.ProfileVersion
		rt.InferenceSettings = CloneResolvedInferenceSettings(resolvedPlan.InferenceSettings)
		rt.ProfileMetadata = CopyMetadataMap(resolvedPlan.ProfileMetadata)
		if resolvedPlan.Runtime != nil {
			rt.SystemPrompt = strings.TrimSpace(resolvedPlan.Runtime.SystemPrompt)
			rt.Middlewares = append([]infruntime.MiddlewareUse(nil), resolvedPlan.Runtime.Middlewares...)
			rt.ToolNames = append([]string(nil), resolvedPlan.Runtime.Tools...)
		}
	}
	rt.RuntimeFingerprint = infruntime.BuildRuntimeFingerprintFromSettings(rt.RuntimeKey, rt.ProfileVersion, ToRuntimeTransport(rt), rt.InferenceSettings)

	return &ConversationPlan{
		ConvID:         convID,
		Prompt:         prompt,
		IdempotencyKey: idempotencyKey,
		Runtime:        rt,
	}, nil
}

// ResolveRuntimePlan resolves the runtime plan from a resolved profile.
func (r *RequestResolver) ResolveRuntimePlan(ctx context.Context, resolved *gepprofiles.ResolvedEngineProfile) (*infruntime.ResolvedRuntimePlan, error) {
	plan, err := infruntime.ResolveRuntimePlan(ctx, r.profileRegistry, resolved, infruntime.ResolveRuntimePlanOptions{
		BaseInferenceSettings: r.baseInferenceSettings,
		BaseRuntime:           defaultWebChatProfileRuntime(),
	})
	if err == nil {
		return plan, nil
	}
	if errors.Is(err, gepprofiles.ErrProfileNotFound) {
		slug := ""
		if resolved != nil {
			slug = resolved.EngineProfileSlug.String()
		}
		return nil, r.toRequestResolutionError(err, slug)
	}
	var validationErr *gepprofiles.ValidationError
	if errors.As(err, &validationErr) {
		return nil, &RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid pinocchio runtime extension", Err: err}
	}
	return nil, err
}

// ResolveProfileRuntime resolves the profile runtime from a resolved profile.
func (r *RequestResolver) ResolveProfileRuntime(ctx context.Context, resolved *gepprofiles.ResolvedEngineProfile) (*infruntime.ProfileRuntime, error) {
	plan, err := r.ResolveRuntimePlan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}
	return plan.Runtime, nil
}

func (r *RequestResolver) resolveRuntimePlan(ctx context.Context, resolved *gepprofiles.ResolvedEngineProfile) (*infruntime.ResolvedRuntimePlan, error) {
	return r.ResolveRuntimePlan(ctx, resolved)
}

func (r *RequestResolver) resolveProfileSlugFromCookie(ctx context.Context, raw string) (gepprofiles.EngineProfileSlug, bool) {
	if r == nil || r.profileRegistry == nil {
		return "", false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, cookieProfile, ok := parseCurrentProfileCookieValue(raw); ok {
		return cookieProfile, true
	}

	legacyProfile, err := gepprofiles.ParseEngineProfileSlug(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	if _, err := r.profileRegistry.GetEngineProfile(ctx, r.defaultRegistrySlug, legacyProfile); err != nil {
		return "", false
	}
	return legacyProfile, true
}

func (r *RequestResolver) toRequestResolutionError(err error, slug string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gepprofiles.ErrProfileNotFound) {
		if strings.TrimSpace(slug) == "" {
			return &RequestResolutionError{Status: http.StatusNotFound, ClientMsg: "profile not found"}
		}
		return &RequestResolutionError{Status: http.StatusNotFound, ClientMsg: "profile not found: " + slug}
	}
	if errors.Is(err, gepprofiles.ErrRegistryNotFound) {
		return &RequestResolutionError{Status: http.StatusNotFound, ClientMsg: "registry not found", Err: err}
	}
	var validationErr *gepprofiles.ValidationError
	if errors.As(err, &validationErr) {
		return &RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: validationErr.Error(), Err: err}
	}
	return &RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile resolution failed", Err: err}
}

// Registry helpers

func BuildBootstrapRegistry(defaultSlug string, profileDefs ...*gepprofiles.EngineProfile) (*gepprofiles.EngineProfileRegistry, error) {
	registrySlug := gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	registry := &gepprofiles.EngineProfileRegistry{
		Slug:     registrySlug,
		Profiles: map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{},
	}

	for _, profile := range profileDefs {
		if profile == nil {
			continue
		}
		clone := profile.Clone()
		if clone == nil {
			continue
		}
		if err := gepprofiles.ValidateEngineProfile(clone); err != nil {
			return nil, err
		}
		registry.Profiles[clone.Slug] = clone
	}

	if strings.TrimSpace(defaultSlug) != "" {
		slug, err := gepprofiles.ParseEngineProfileSlug(defaultSlug)
		if err != nil {
			return nil, err
		}
		registry.DefaultEngineProfileSlug = slug
	}

	if len(registry.Profiles) > 0 {
		if registry.DefaultEngineProfileSlug.IsZero() {
			registry.DefaultEngineProfileSlug = firstProfileSlug(registry.Profiles)
		}
		if _, ok := registry.Profiles[registry.DefaultEngineProfileSlug]; !ok {
			registry.DefaultEngineProfileSlug = firstProfileSlug(registry.Profiles)
		}
	}

	if err := gepprofiles.ValidateRegistry(registry); err != nil {
		return nil, err
	}
	return registry, nil
}

func NewInMemoryProfileService(defaultSlug string, profileDefs ...*gepprofiles.EngineProfile) (gepprofiles.Registry, error) {
	registrySlug := gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	registry, err := BuildBootstrapRegistry(defaultSlug, profileDefs...)
	if err != nil {
		return nil, err
	}

	store := gepprofiles.NewInMemoryEngineProfileStore()
	if err := store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{Actor: "web-chat", Source: "builtin"}); err != nil {
		return nil, err
	}
	return gepprofiles.NewStoreRegistry(store, registrySlug)
}

func NewSQLiteProfileService(dsn string, dbPath string, defaultSlug string, profileDefs ...*gepprofiles.EngineProfile) (gepprofiles.Registry, func(), error) {
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

	store, err := gepprofiles.NewSQLiteEngineProfileStore(dsn, registrySlug)
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
		registry, err := BuildBootstrapRegistry(defaultSlug, profileDefs...)
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

func firstProfileSlug(profiles map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile) gepprofiles.EngineProfileSlug {
	slugs := make([]gepprofiles.EngineProfileSlug, 0, len(profiles))
	for slug := range profiles {
		slugs = append(slugs, slug)
	}
	sort.Slice(slugs, func(i, j int) bool { return slugs[i] < slugs[j] })
	if len(slugs) == 0 {
		return ""
	}
	return slugs[0]
}

func runtimeKeyFromResolvedProfile(resolved *gepprofiles.ResolvedEngineProfile) string {
	if resolved == nil {
		return "default"
	}
	if slug := strings.TrimSpace(resolved.EngineProfileSlug.String()); slug != "" {
		return slug
	}
	return "default"
}

func defaultWebChatProfileRuntime() *infruntime.ProfileRuntime {
	return &infruntime.ProfileRuntime{
		Middlewares: []infruntime.MiddlewareUse{
			{Name: "agentmode"},
		},
	}
}

func ToRuntimeTransport(runtime *ResolvedRuntime) *infruntime.ProfileRuntime {
	if runtime == nil {
		return nil
	}
	return &infruntime.ProfileRuntime{
		SystemPrompt: strings.TrimSpace(runtime.SystemPrompt),
		Middlewares:  append([]infruntime.MiddlewareUse(nil), runtime.Middlewares...),
		Tools:        append([]string(nil), runtime.ToolNames...),
	}
}

func parseCurrentProfileCookieValue(raw string) (gepprofiles.RegistrySlug, gepprofiles.EngineProfileSlug, bool) {
	parts := strings.SplitN(strings.TrimSpace(raw), "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	registrySlug, err := gepprofiles.ParseRegistrySlug(parts[0])
	if err != nil {
		return "", "", false
	}
	profileSlug, err := gepprofiles.ParseEngineProfileSlug(parts[1])
	if err != nil {
		return "", "", false
	}
	return registrySlug, profileSlug, true
}

func CloneResolvedInferenceSettings(in *aisettings.InferenceSettings) *aisettings.InferenceSettings {
	if in == nil {
		return nil
	}
	return in.Clone()
}

func CopyMetadataMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
