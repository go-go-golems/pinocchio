package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/google/uuid"

	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
)

const defaultWebChatRegistrySlug = "default"

func newInMemoryProfileRegistry(defaultSlug string, profileDefs ...*gepprofiles.Profile) (gepprofiles.Registry, error) {
	registrySlug := gepprofiles.MustRegistrySlug(defaultWebChatRegistrySlug)
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

	store := gepprofiles.NewInMemoryProfileStore()
	if err := store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{Actor: "web-chat", Source: "builtin"}); err != nil {
		return nil, err
	}
	return gepprofiles.NewStoreRegistry(store, registrySlug)
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

type webChatProfileResolver struct {
	profileRegistry gepprofiles.Registry
	registrySlug    gepprofiles.RegistrySlug
}

func newWebChatProfileResolver(profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug) *webChatProfileResolver {
	if registrySlug.IsZero() {
		registrySlug = gepprofiles.MustRegistrySlug(defaultWebChatRegistrySlug)
	}
	return &webChatProfileResolver{profileRegistry: profileRegistry, registrySlug: registrySlug}
}

func (r *webChatProfileResolver) Resolve(req *http.Request) (webhttp.ConversationRequestPlan, error) {
	if req == nil {
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request"}
	}

	switch req.Method {
	case http.MethodGet:
		return r.resolveWS(req)
	case http.MethodPost:
		return r.resolveChat(req)
	default:
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{Status: http.StatusMethodNotAllowed, ClientMsg: "method not allowed"}
	}
}

func (r *webChatProfileResolver) resolveWS(req *http.Request) (webhttp.ConversationRequestPlan, error) {
	convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
	if convID == "" {
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "missing conv_id"}
	}

	slug, profile, err := r.resolveProfile(req, "")
	if err != nil {
		return webhttp.ConversationRequestPlan{}, err
	}
	overrides := baseOverridesForProfile(profile)

	return webhttp.ConversationRequestPlan{
		ConvID:     convID,
		RuntimeKey: slug.String(),
		Overrides:  overrides,
	}, nil
}

func (r *webChatProfileResolver) resolveChat(req *http.Request) (webhttp.ConversationRequestPlan, error) {
	var body webhttp.ChatRequestBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request", Err: err}
	}
	if body.Prompt == "" && body.Text != "" {
		body.Prompt = body.Text
	}

	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}

	pathSlug := runtimeKeyFromPath(req)
	slug, profile, err := r.resolveProfile(req, pathSlug)
	if err != nil {
		return webhttp.ConversationRequestPlan{}, err
	}

	overrides, err := mergeOverrides(profile, body.Overrides)
	if err != nil {
		return webhttp.ConversationRequestPlan{}, err
	}

	return webhttp.ConversationRequestPlan{
		ConvID:         convID,
		RuntimeKey:     slug.String(),
		Overrides:      overrides,
		Prompt:         body.Prompt,
		IdempotencyKey: strings.TrimSpace(body.IdempotencyKey),
	}, nil
}

func (r *webChatProfileResolver) resolveProfile(req *http.Request, pathSlug string) (gepprofiles.ProfileSlug, *gepprofiles.Profile, error) {
	if r == nil || r.profileRegistry == nil {
		return "", nil, &webhttp.RequestResolutionError{Status: http.StatusInternalServerError, ClientMsg: "profile resolver is not configured"}
	}

	slugRaw := strings.TrimSpace(pathSlug)
	if slugRaw == "" && req != nil {
		slugRaw = strings.TrimSpace(req.URL.Query().Get("profile"))
	}
	if slugRaw == "" && req != nil {
		slugRaw = strings.TrimSpace(req.URL.Query().Get("runtime"))
	}
	if slugRaw == "" && req != nil {
		if ck, err := req.Cookie("chat_profile"); err == nil && ck != nil {
			slugRaw = strings.TrimSpace(ck.Value)
		}
	}

	ctx := context.Background()
	if strings.TrimSpace(slugRaw) == "" {
		s, err := r.resolveDefaultProfileSlug(ctx)
		if err != nil {
			return "", nil, r.toRequestResolutionError(err, "")
		}
		slugRaw = s.String()
	}

	slug, err := gepprofiles.ParseProfileSlug(slugRaw)
	if err != nil {
		return "", nil, &webhttp.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "invalid profile: " + slugRaw, Err: err}
	}

	profile, err := r.profileRegistry.GetProfile(ctx, r.registrySlug, slug)
	if err != nil {
		return "", nil, r.toRequestResolutionError(err, slugRaw)
	}
	return slug, profile, nil
}

func (r *webChatProfileResolver) resolveDefaultProfileSlug(ctx context.Context) (gepprofiles.ProfileSlug, error) {
	registry, err := r.profileRegistry.GetRegistry(ctx, r.registrySlug)
	if err != nil {
		return "", err
	}
	if registry != nil && !registry.DefaultProfileSlug.IsZero() {
		return registry.DefaultProfileSlug, nil
	}
	return gepprofiles.MustProfileSlug("default"), nil
}

func (r *webChatProfileResolver) toRequestResolutionError(err error, slug string) error {
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

func (r *webChatProfileResolver) profileExists(ctx context.Context, slug gepprofiles.ProfileSlug) bool {
	_, err := r.profileRegistry.GetProfile(ctx, r.registrySlug, slug)
	return err == nil
}

func baseOverridesForProfile(p *gepprofiles.Profile) map[string]any {
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

func mergeOverrides(profile *gepprofiles.Profile, requestOverrides map[string]any) (map[string]any, error) {
	merged := baseOverridesForProfile(profile)
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

func runtimeKeyFromPath(req *http.Request) string {
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

func registerProfileHandlers(mux *http.ServeMux, resolver *webChatProfileResolver) {
	if mux == nil || resolver == nil || resolver.profileRegistry == nil {
		return
	}

	mux.HandleFunc("/api/chat/profiles", func(w http.ResponseWriter, _ *http.Request) {
		type profileInfo struct {
			Slug          string `json:"slug"`
			DefaultPrompt string `json:"default_prompt"`
		}

		profiles_, err := resolver.profileRegistry.ListProfiles(context.Background(), resolver.registrySlug)
		if err != nil {
			http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
			return
		}
		items := make([]profileInfo, 0, len(profiles_))
		for _, p := range profiles_ {
			if p == nil {
				continue
			}
			items = append(items, profileInfo{
				Slug:          p.Slug.String(),
				DefaultPrompt: p.Runtime.SystemPrompt,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	})

	mux.HandleFunc("/api/chat/profile", func(w http.ResponseWriter, req *http.Request) {
		type profilePayload struct {
			Slug    string `json:"slug"`
			Profile string `json:"profile"`
		}
		writeJSON := func(payload any) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		}

		switch req.Method {
		case http.MethodGet:
			slug := gepprofiles.ProfileSlug("")
			if ck, err := req.Cookie("chat_profile"); err == nil && ck != nil {
				if parsed, err := gepprofiles.ParseProfileSlug(strings.TrimSpace(ck.Value)); err == nil && resolver.profileExists(context.Background(), parsed) {
					slug = parsed
				}
			}
			if slug.IsZero() {
				defaultSlug, err := resolver.resolveDefaultProfileSlug(context.Background())
				if err != nil {
					http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
					return
				}
				slug = defaultSlug
			}
			writeJSON(profilePayload{Slug: slug.String()})
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
				Name:     "chat_profile",
				Value:    slug.String(),
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Secure:   true,
				HttpOnly: true,
			})
			writeJSON(profilePayload{Slug: slug.String()})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
