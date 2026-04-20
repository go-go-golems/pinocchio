package main

import (
	"encoding/json"
	"net/http"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/profiles"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/google/uuid"
)

// ProfileRequestResolver is the legacy wrapper around profiles.RequestResolver.
// It is kept only for backward compatibility with legacy tests and will be removed
// once all callers migrate to profiles.RequestResolver directly.
type ProfileRequestResolver struct {
	*profiles.RequestResolver
}

func newProfileRequestResolver(profileRegistry gepprofiles.Registry, defaultRegistry gepprofiles.RegistrySlug, baseInferenceSettings *aisettings.InferenceSettings) *ProfileRequestResolver {
	return &ProfileRequestResolver{
		RequestResolver: profiles.NewRequestResolver(profileRegistry, defaultRegistry, baseInferenceSettings),
	}
}

func newInMemoryProfileService(defaultSlug string, profileDefs ...*gepprofiles.EngineProfile) (gepprofiles.Registry, error) {
	return profiles.NewInMemoryProfileService(defaultSlug, profileDefs...)
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
	profileSlug, err := r.ResolveProfileSelection(req.Context(), "", "", "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	registrySlug, err := r.ResolveRegistrySelection("", "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedProfile, err := r.ResolveEffectiveProfile(req.Context(), registrySlug, profileSlug)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	plan, err := r.BuildConversationPlan(req.Context(), convID, "", "", resolvedProfile)
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
	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}
	profileSlug, err := r.ResolveProfileSelection(req.Context(), "", body.Profile, "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	registrySlug, err := r.ResolveRegistrySelection(body.Registry, "", "")
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	resolvedProfile, err := r.ResolveEffectiveProfile(req.Context(), registrySlug, profileSlug)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	plan, err := r.BuildConversationPlan(req.Context(), convID, body.Prompt, strings.TrimSpace(body.IdempotencyKey), resolvedProfile)
	if err != nil {
		return webhttp.ResolvedConversationRequest{}, err
	}
	return toResolvedConversationRequest(plan), nil
}

func toResolvedConversationRequest(plan *profiles.ConversationPlan) webhttp.ResolvedConversationRequest {
	if plan == nil || plan.Runtime == nil {
		return webhttp.ResolvedConversationRequest{}
	}
	return webhttp.ResolvedConversationRequest{
		ConvID:                    plan.ConvID,
		RuntimeKey:                plan.Runtime.RuntimeKey,
		RuntimeFingerprint:        plan.Runtime.RuntimeFingerprint,
		ProfileVersion:            plan.Runtime.ProfileVersion,
		ResolvedInferenceSettings: profiles.CloneResolvedInferenceSettings(plan.Runtime.InferenceSettings),
		ResolvedRuntime:           profiles.ToRuntimeTransport(plan.Runtime),
		ProfileMetadata:           profiles.CopyMetadataMap(plan.Runtime.ProfileMetadata),
		Prompt:                    plan.Prompt,
		IdempotencyKey:            plan.IdempotencyKey,
	}
}

func registerProfileAPIHandlers(mux *http.ServeMux, resolver *profiles.RequestResolver) {
	if mux == nil || resolver == nil || resolver.Registry() == nil {
		return
	}
	profiles.RegisterAPIHandlers(mux, resolver.Registry(), profiles.APIOptions{
		DefaultRegistrySlug:             resolver.DefaultRegistrySlug(),
		EnableCurrentProfileCookieRoute: true,
		CurrentProfileCookieName:        "chat_profile",
		ExtensionSchemas: []profiles.ExtensionSchemaDocument{
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
