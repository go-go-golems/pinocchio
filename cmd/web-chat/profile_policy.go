package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
)

type chatProfile struct {
	Slug           string
	DefaultPrompt  string
	DefaultTools   []string
	DefaultMws     []webchat.MiddlewareUse
	AllowOverrides bool
}

type chatProfileRegistry struct {
	bySlug      map[string]*chatProfile
	order       []string
	defaultSlug string
}

func newChatProfileRegistry(defaultSlug string, profiles ...*chatProfile) *chatProfileRegistry {
	reg := &chatProfileRegistry{
		bySlug:      map[string]*chatProfile{},
		order:       []string{},
		defaultSlug: strings.TrimSpace(defaultSlug),
	}
	for _, p := range profiles {
		if p == nil || strings.TrimSpace(p.Slug) == "" {
			continue
		}
		slug := strings.TrimSpace(p.Slug)
		cp := *p
		cp.Slug = slug
		reg.bySlug[slug] = &cp
		reg.order = append(reg.order, slug)
	}
	if reg.defaultSlug == "" || reg.bySlug[reg.defaultSlug] == nil {
		if len(reg.order) > 0 {
			reg.defaultSlug = reg.order[0]
		} else {
			reg.defaultSlug = "default"
		}
	}
	return reg
}

func (r *chatProfileRegistry) get(slug string) (*chatProfile, bool) {
	if r == nil {
		return nil, false
	}
	s := strings.TrimSpace(slug)
	if s == "" {
		return nil, false
	}
	p, ok := r.bySlug[s]
	return p, ok
}

func (r *chatProfileRegistry) list() []*chatProfile {
	if r == nil {
		return nil
	}
	out := make([]*chatProfile, 0, len(r.order))
	for _, slug := range r.order {
		if p := r.bySlug[slug]; p != nil {
			out = append(out, p)
		}
	}
	return out
}

func (r *chatProfileRegistry) resolveDefault() string {
	if r == nil {
		return "default"
	}
	if strings.TrimSpace(r.defaultSlug) != "" {
		return r.defaultSlug
	}
	if len(r.order) > 0 {
		return r.order[0]
	}
	return "default"
}

type webChatProfileResolver struct {
	profiles *chatProfileRegistry
}

func newWebChatProfileResolver(profiles *chatProfileRegistry) *webChatProfileResolver {
	return &webChatProfileResolver{profiles: profiles}
}

func (r *webChatProfileResolver) Resolve(req *http.Request) (webchat.ConversationRequestPlan, error) {
	if req == nil {
		return webchat.ConversationRequestPlan{}, &webchat.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request"}
	}

	switch req.Method {
	case http.MethodGet:
		return r.resolveWS(req)
	case http.MethodPost:
		return r.resolveChat(req)
	default:
		return webchat.ConversationRequestPlan{}, &webchat.RequestResolutionError{Status: http.StatusMethodNotAllowed, ClientMsg: "method not allowed"}
	}
}

func (r *webChatProfileResolver) resolveWS(req *http.Request) (webchat.ConversationRequestPlan, error) {
	convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
	if convID == "" {
		return webchat.ConversationRequestPlan{}, &webchat.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "missing conv_id"}
	}

	slug, profile, err := r.resolveProfile(req, "")
	if err != nil {
		return webchat.ConversationRequestPlan{}, err
	}
	overrides := baseOverridesForProfile(profile)

	return webchat.ConversationRequestPlan{
		ConvID:     convID,
		RuntimeKey: slug,
		Overrides:  overrides,
	}, nil
}

func (r *webChatProfileResolver) resolveChat(req *http.Request) (webchat.ConversationRequestPlan, error) {
	var body webchat.ChatRequestBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return webchat.ConversationRequestPlan{}, &webchat.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request", Err: err}
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
		return webchat.ConversationRequestPlan{}, err
	}

	overrides, err := mergeOverrides(profile, body.Overrides)
	if err != nil {
		return webchat.ConversationRequestPlan{}, err
	}

	return webchat.ConversationRequestPlan{
		ConvID:         convID,
		RuntimeKey:     slug,
		Overrides:      overrides,
		Prompt:         body.Prompt,
		IdempotencyKey: strings.TrimSpace(body.IdempotencyKey),
	}, nil
}

func (r *webChatProfileResolver) resolveProfile(req *http.Request, pathSlug string) (string, *chatProfile, error) {
	slug := strings.TrimSpace(pathSlug)
	if slug == "" && req != nil {
		slug = strings.TrimSpace(req.URL.Query().Get("profile"))
	}
	if slug == "" && req != nil {
		slug = strings.TrimSpace(req.URL.Query().Get("runtime"))
	}
	if slug == "" && req != nil {
		if ck, err := req.Cookie("chat_profile"); err == nil && ck != nil {
			slug = strings.TrimSpace(ck.Value)
		}
	}
	if slug == "" {
		slug = r.profiles.resolveDefault()
	}
	p, ok := r.profiles.get(slug)
	if !ok || p == nil {
		return "", nil, &webchat.RequestResolutionError{Status: http.StatusNotFound, ClientMsg: "profile not found: " + slug}
	}
	return slug, p, nil
}

func baseOverridesForProfile(p *chatProfile) map[string]any {
	if p == nil {
		return nil
	}
	overrides := map[string]any{}
	if strings.TrimSpace(p.DefaultPrompt) != "" {
		overrides["system_prompt"] = p.DefaultPrompt
	}
	if len(p.DefaultMws) > 0 {
		mws := make([]any, 0, len(p.DefaultMws))
		for _, mw := range p.DefaultMws {
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
	if len(p.DefaultTools) > 0 {
		tools := make([]any, 0, len(p.DefaultTools))
		for _, t := range p.DefaultTools {
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

func mergeOverrides(profile *chatProfile, requestOverrides map[string]any) (map[string]any, error) {
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
	if hasEngineOverride && profile != nil && !profile.AllowOverrides {
		return nil, &webchat.RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "profile does not allow engine overrides"}
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

func registerProfileHandlers(router *webchat.Router, profiles *chatProfileRegistry) {
	if router == nil || profiles == nil {
		return
	}

	router.HandleFunc("/api/chat/profiles", func(w http.ResponseWriter, _ *http.Request) {
		type profileInfo struct {
			Slug          string `json:"slug"`
			DefaultPrompt string `json:"default_prompt"`
		}
		items := make([]profileInfo, 0, len(profiles.order))
		for _, p := range profiles.list() {
			if p == nil {
				continue
			}
			items = append(items, profileInfo{
				Slug:          p.Slug,
				DefaultPrompt: p.DefaultPrompt,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	})

	router.HandleFunc("/api/chat/profile", func(w http.ResponseWriter, req *http.Request) {
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
			slug := ""
			if ck, err := req.Cookie("chat_profile"); err == nil && ck != nil {
				slug = strings.TrimSpace(ck.Value)
			}
			if _, ok := profiles.get(slug); !ok {
				slug = profiles.resolveDefault()
			}
			writeJSON(profilePayload{Slug: slug})
		case http.MethodPost:
			var body profilePayload
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			slug := strings.TrimSpace(body.Slug)
			if slug == "" {
				slug = strings.TrimSpace(body.Profile)
			}
			if _, ok := profiles.get(slug); !ok {
				http.Error(w, "profile not found", http.StatusNotFound)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "chat_profile", Value: slug, Path: "/", SameSite: http.SameSiteLaxMode})
			writeJSON(profilePayload{Slug: slug})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
