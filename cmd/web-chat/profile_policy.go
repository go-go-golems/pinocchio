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

	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
)

const (
	defaultRegistrySlug      = "default"
	currentProfileCookieName = "chat_profile"
)

func buildBootstrapRegistry(defaultSlug string, profileDefs ...*gepprofiles.EngineProfile) (*gepprofiles.EngineProfileRegistry, error) {
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

func newInMemoryProfileService(defaultSlug string, profileDefs ...*gepprofiles.EngineProfile) (gepprofiles.Registry, error) {
	registrySlug := gepprofiles.MustRegistrySlug(defaultRegistrySlug)
	registry, err := buildBootstrapRegistry(defaultSlug, profileDefs...)
	if err != nil {
		return nil, err
	}

	store := gepprofiles.NewInMemoryEngineProfileStore()
	if err := store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{Actor: "web-chat", Source: "builtin"}); err != nil {
		return nil, err
	}
	return gepprofiles.NewStoreRegistry(store, registrySlug)
}

func newSQLiteProfileService(
	dsn string,
	dbPath string,
	defaultSlug string,
	profileDefs ...*gepprofiles.EngineProfile,
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

type ProfileRequestResolver struct {
	profileRegistry       gepprofiles.Registry
	defaultRegistrySlug   gepprofiles.RegistrySlug
	baseInferenceSettings *aisettings.InferenceSettings
}

type resolvedWebChatConversationPlan struct {
	ConvID         string
	Prompt         string
	IdempotencyKey string
	Runtime        *resolvedWebChatRuntime
}

type resolvedWebChatRuntime struct {
	SystemPrompt       string
	Middlewares        []infruntime.MiddlewareUse
	ToolNames          []string
	RuntimeKey         string
	RuntimeFingerprint string
	ProfileVersion     uint64
	InferenceSettings  *aisettings.InferenceSettings
	ProfileMetadata    map[string]any
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
	plan, err := r.buildConversationPlan(context.Background(), convID, "", "", resolvedProfile)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	return toResolvedConversationRequest(plan), nil
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
	plan, err := r.buildConversationPlan(context.Background(), convID, body.Prompt, strings.TrimSpace(body.IdempotencyKey), resolvedProfile)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	return toResolvedConversationRequest(plan), nil
}

func (r *ProfileRequestResolver) resolveProfileSelection(
	req *http.Request,
	pathSlug string,
	bodyProfileRaw string,
) (gepprofiles.EngineProfileSlug, error) {
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
	if r == nil || r.profileRegistry == nil {
		return "", &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "profile selection requires configured profile registries",
		}
	}

	slug, err := gepprofiles.ParseEngineProfileSlug(slugRaw)
	if err != nil {
		return "", &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid profile: " + slugRaw, Err: err}
	}
	return slug, nil
}

func (r *ProfileRequestResolver) resolveEffectiveProfile(
	ctx context.Context,
	registrySlug gepprofiles.RegistrySlug,
	profileSlug gepprofiles.EngineProfileSlug,
) (*gepprofiles.ResolvedEngineProfile, error) {
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

func runtimeKeyFromResolvedProfile(resolved *gepprofiles.ResolvedEngineProfile) string {
	if resolved == nil {
		return "default"
	}
	if slug := strings.TrimSpace(resolved.EngineProfileSlug.String()); slug != "" {
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

func resolvedInferenceSettingsForRequest(resolved *gepprofiles.ResolvedEngineProfile, base *aisettings.InferenceSettings) (*aisettings.InferenceSettings, error) {
	if resolved == nil || resolved.InferenceSettings == nil {
		return cloneResolvedInferenceSettings(base), nil
	}
	merged, err := gepprofiles.MergeInferenceSettings(base, resolved.InferenceSettings)
	if err != nil {
		return nil, err
	}
	return merged, nil
}

func (r *ProfileRequestResolver) buildConversationPlan(
	ctx context.Context,
	convID string,
	prompt string,
	idempotencyKey string,
	resolvedProfile *gepprofiles.ResolvedEngineProfile,
) (*resolvedWebChatConversationPlan, error) {
	resolvedRuntime, err := r.resolveProfileRuntime(ctx, resolvedProfile)
	if err != nil {
		return nil, err
	}
	runtimeKey := runtimeKeyFromResolvedProfile(resolvedProfile)
	var (
		profileVersion  uint64
		profileMetadata map[string]any
	)
	if resolvedProfile != nil {
		profileVersion = profileVersionFromResolvedMetadata(resolvedProfile.Metadata)
		profileMetadata = copyMetadataMap(resolvedProfile.Metadata)
	}
	inferenceSettings, err := resolvedInferenceSettingsForRequest(resolvedProfile, r.baseInferenceSettings)
	if err != nil {
		return nil, &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "failed to merge inference settings", Err: err}
	}

	runtime := &resolvedWebChatRuntime{
		SystemPrompt:       "",
		Middlewares:        nil,
		ToolNames:          nil,
		RuntimeKey:         runtimeKey,
		RuntimeFingerprint: "",
		ProfileVersion:     profileVersion,
		InferenceSettings:  inferenceSettings,
		ProfileMetadata:    profileMetadata,
	}
	if resolvedRuntime != nil {
		runtime.SystemPrompt = strings.TrimSpace(resolvedRuntime.SystemPrompt)
		runtime.Middlewares = append([]infruntime.MiddlewareUse(nil), resolvedRuntime.Middlewares...)
		runtime.ToolNames = append([]string(nil), resolvedRuntime.Tools...)
	}
	runtime.RuntimeFingerprint = buildResolvedRuntimeFingerprint(runtime.RuntimeKey, runtime.ProfileVersion, runtime, runtime.InferenceSettings)

	return &resolvedWebChatConversationPlan{
		ConvID:         convID,
		Prompt:         prompt,
		IdempotencyKey: idempotencyKey,
		Runtime:        runtime,
	}, nil
}

func (r *ProfileRequestResolver) resolveProfileRuntime(
	ctx context.Context,
	resolved *gepprofiles.ResolvedEngineProfile,
) (*infruntime.ProfileRuntime, error) {
	runtime := defaultWebChatProfileRuntime()
	if r == nil || r.profileRegistry == nil || resolved == nil {
		return runtime, nil
	}
	for _, ref := range runtimeStackRefsFromResolvedProfile(resolved) {
		profile, err := r.profileRegistry.GetEngineProfile(ctx, ref.registrySlug, ref.profileSlug)
		if err != nil {
			return nil, r.toRequestResolutionError(err, ref.profileSlug.String())
		}
		profileRuntime, _, err := infruntime.ProfileRuntimeFromEngineProfile(profile)
		if err != nil {
			return nil, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid pinocchio runtime extension", Err: err}
		}
		runtime = mergeWebChatProfileRuntime(runtime, profileRuntime)
	}
	return runtime, nil
}

func defaultWebChatProfileRuntime() *infruntime.ProfileRuntime {
	return &infruntime.ProfileRuntime{
		Middlewares: []infruntime.MiddlewareUse{
			{Name: "agentmode"},
		},
	}
}

type runtimeStackRef struct {
	registrySlug gepprofiles.RegistrySlug
	profileSlug  gepprofiles.EngineProfileSlug
}

func runtimeStackRefsFromResolvedProfile(resolved *gepprofiles.ResolvedEngineProfile) []runtimeStackRef {
	if resolved == nil {
		return nil
	}

	refs := parseRuntimeStackRefsFromMetadata(resolved.Metadata)
	if len(refs) > 0 {
		return refs
	}

	return []runtimeStackRef{{
		registrySlug: resolved.RegistrySlug,
		profileSlug:  resolved.EngineProfileSlug,
	}}
}

func parseRuntimeStackRefsFromMetadata(metadata map[string]any) []runtimeStackRef {
	rawLineage, ok := metadata["profile.stack.lineage"]
	if !ok {
		return nil
	}

	items, ok := rawLineage.([]map[string]any)
	if ok {
		refs := make([]runtimeStackRef, 0, len(items))
		for _, item := range items {
			ref, ok := runtimeStackRefFromMetadataItem(item)
			if ok {
				refs = append(refs, ref)
			}
		}
		return refs
	}

	rawItems, ok := rawLineage.([]any)
	if !ok {
		return nil
	}

	refs := make([]runtimeStackRef, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		ref, ok := runtimeStackRefFromMetadataItem(item)
		if ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

func runtimeStackRefFromMetadataItem(item map[string]any) (runtimeStackRef, bool) {
	registryRaw, _ := item["registry_slug"].(string)
	profileRaw, _ := item["profile_slug"].(string)
	if strings.TrimSpace(registryRaw) == "" || strings.TrimSpace(profileRaw) == "" {
		return runtimeStackRef{}, false
	}

	registrySlug, err := gepprofiles.ParseRegistrySlug(registryRaw)
	if err != nil {
		return runtimeStackRef{}, false
	}
	profileSlug, err := gepprofiles.ParseEngineProfileSlug(profileRaw)
	if err != nil {
		return runtimeStackRef{}, false
	}

	return runtimeStackRef{
		registrySlug: registrySlug,
		profileSlug:  profileSlug,
	}, true
}

func mergeWebChatProfileRuntime(base *infruntime.ProfileRuntime, overlay *infruntime.ProfileRuntime) *infruntime.ProfileRuntime {
	if base == nil && overlay == nil {
		return nil
	}
	if base == nil {
		return overlay.Clone()
	}
	if overlay == nil {
		return base.Clone()
	}

	merged := base.Clone()
	if prompt := strings.TrimSpace(overlay.SystemPrompt); prompt != "" {
		merged.SystemPrompt = prompt
	}
	merged.Middlewares = mergeRuntimeMiddlewares(merged.Middlewares, overlay.Middlewares)
	merged.Tools = mergeRuntimeTools(merged.Tools, overlay.Tools)
	return merged
}

func mergeRuntimeMiddlewares(base []infruntime.MiddlewareUse, overlay []infruntime.MiddlewareUse) []infruntime.MiddlewareUse {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}

	out := make([]infruntime.MiddlewareUse, 0, len(base)+len(overlay))
	positions := map[string]int{}

	appendUse := func(use infruntime.MiddlewareUse) {
		key := runtimeMiddlewareKey(use)
		clone := infruntime.MiddlewareUse{
			Name:    strings.TrimSpace(use.Name),
			ID:      strings.TrimSpace(use.ID),
			Enabled: cloneBoolPtr(use.Enabled),
			Config:  cloneStringAnyMap(use.Config),
		}
		if idx, ok := positions[key]; ok {
			out[idx] = clone
			return
		}
		positions[key] = len(out)
		out = append(out, clone)
	}

	for _, use := range base {
		appendUse(use)
	}
	for _, use := range overlay {
		appendUse(use)
	}

	return out
}

func runtimeMiddlewareKey(use infruntime.MiddlewareUse) string {
	name := strings.TrimSpace(use.Name)
	id := strings.TrimSpace(use.ID)
	if id == "" {
		return name
	}
	return name + "\x00" + id
}

func mergeRuntimeTools(base []string, overlay []string) []string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}

	out := make([]string, 0, len(base)+len(overlay))
	seen := map[string]struct{}{}
	appendTool := func(tool string) {
		name := strings.TrimSpace(tool)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	for _, tool := range base {
		appendTool(tool)
	}
	for _, tool := range overlay {
		appendTool(tool)
	}

	return out
}

func buildResolvedRuntimeFingerprint(
	runtimeKey string,
	profileVersion uint64,
	runtime *resolvedWebChatRuntime,
	inferenceSettings *aisettings.InferenceSettings,
) string {
	if strings.TrimSpace(runtimeKey) == "" {
		runtimeKey = "default"
	}
	if runtime == nil {
		return buildRuntimeFingerprint(runtimeKey, profileVersion, "", nil, nil, inferenceSettings)
	}
	middlewares := append([]infruntime.MiddlewareUse(nil), runtime.Middlewares...)
	tools := append([]string(nil), runtime.ToolNames...)
	return buildRuntimeFingerprint(runtimeKey, profileVersion, strings.TrimSpace(runtime.SystemPrompt), middlewares, tools, inferenceSettings)
}

func toResolvedConversationRequest(plan *resolvedWebChatConversationPlan) webhttp.ResolvedConversationRequest {
	if plan == nil || plan.Runtime == nil {
		return webhttp.ResolvedConversationRequest{}
	}
	return webhttp.ResolvedConversationRequest{
		ConvID:                    plan.ConvID,
		RuntimeKey:                plan.Runtime.RuntimeKey,
		RuntimeFingerprint:        plan.Runtime.RuntimeFingerprint,
		ProfileVersion:            plan.Runtime.ProfileVersion,
		ResolvedInferenceSettings: cloneResolvedInferenceSettings(plan.Runtime.InferenceSettings),
		ResolvedRuntime:           toRuntimeTransport(plan.Runtime),
		ProfileMetadata:           copyMetadataMap(plan.Runtime.ProfileMetadata),
		Prompt:                    plan.Prompt,
		IdempotencyKey:            plan.IdempotencyKey,
	}
}

func toRuntimeTransport(runtime *resolvedWebChatRuntime) *infruntime.ProfileRuntime {
	if runtime == nil {
		return nil
	}
	return &infruntime.ProfileRuntime{
		SystemPrompt: strings.TrimSpace(runtime.SystemPrompt),
		Middlewares:  append([]infruntime.MiddlewareUse(nil), runtime.Middlewares...),
		Tools:        append([]string(nil), runtime.ToolNames...),
	}
}

func (r *ProfileRequestResolver) resolveRegistrySelection(req *http.Request, bodyRegistryRaw string) (gepprofiles.RegistrySlug, error) {
	registryRaw := strings.TrimSpace(bodyRegistryRaw)
	if registryRaw == "" && req != nil {
		registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
	}
	if registryRaw == "" && req != nil && r != nil && r.profileRegistry != nil {
		if ck, err := req.Cookie(currentProfileCookieName); err == nil && ck != nil {
			if cookieRegistry, _, ok := parseCurrentProfileCookieValue(strings.TrimSpace(ck.Value)); ok {
				registryRaw = cookieRegistry.String()
			}
		}
	}
	if registryRaw == "" {
		if r == nil {
			return "", nil
		}
		return r.defaultRegistrySlug, nil
	}
	if r == nil || r.profileRegistry == nil {
		return "", &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "registry selection requires configured profile registries",
		}
	}
	registrySlug, err := gepprofiles.ParseRegistrySlug(registryRaw)
	if err != nil {
		return "", &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid registry: " + registryRaw, Err: err}
	}
	return registrySlug, nil
}

func (r *ProfileRequestResolver) resolveProfileSlugFromCookie(ctx context.Context, raw string) (gepprofiles.EngineProfileSlug, bool) {
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
