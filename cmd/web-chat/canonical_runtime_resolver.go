package main

import (
	"context"
	"net/http"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	appserver "github.com/go-go-golems/pinocchio/cmd/web-chat/app"
	chatapp "github.com/go-go-golems/pinocchio/pkg/evtstream/apps/chat"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

type canonicalRuntimeResolver struct {
	requestResolver *ProfileRequestResolver
	runtimeComposer infruntime.RuntimeBuilder
}

func newCanonicalRuntimeResolver(requestResolver *ProfileRequestResolver, runtimeComposer infruntime.RuntimeBuilder) appserver.RuntimeResolver {
	if requestResolver == nil || runtimeComposer == nil {
		return nil
	}
	return &canonicalRuntimeResolver{
		requestResolver: requestResolver,
		runtimeComposer: runtimeComposer,
	}
}

func (r *canonicalRuntimeResolver) Resolve(ctx context.Context, req *http.Request, profile string, registry string) (*chatapp.ResolvedRuntime, error) {
	if r == nil || r.requestResolver == nil || r.runtimeComposer == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	profileSlug, err := r.requestResolver.resolveProfileSelection(req, "", profile)
	if err != nil {
		return nil, err
	}
	registrySlug, err := r.requestResolver.resolveRegistrySelection(req, registry)
	if err != nil {
		return nil, err
	}
	resolvedProfile, err := r.resolveEffectiveProfile(ctx, registrySlug, profileSlug)
	if err != nil {
		return nil, err
	}
	plan, err := r.requestResolver.buildConversationPlan(ctx, "", "", "", resolvedProfile)
	if err != nil {
		return nil, err
	}
	if plan == nil || plan.Runtime == nil {
		return nil, nil
	}
	composed, err := r.runtimeComposer.Compose(ctx, infruntime.ConversationRuntimeRequest{
		ConvID:                     "",
		ProfileKey:                 plan.Runtime.RuntimeKey,
		ProfileVersion:             plan.Runtime.ProfileVersion,
		ResolvedInferenceSettings:  cloneResolvedInferenceSettings(plan.Runtime.InferenceSettings),
		ResolvedProfileRuntime:     toRuntimeTransport(plan.Runtime),
		ResolvedProfileFingerprint: plan.Runtime.RuntimeFingerprint,
	})
	if err != nil {
		return nil, err
	}
	return &chatapp.ResolvedRuntime{ComposedRuntime: composed}, nil
}

func (r *canonicalRuntimeResolver) resolveEffectiveProfile(
	ctx context.Context,
	registrySlug gepprofiles.RegistrySlug,
	profileSlug gepprofiles.EngineProfileSlug,
) (*gepprofiles.ResolvedEngineProfile, error) {
	if r == nil || r.requestResolver == nil {
		return nil, nil
	}
	return r.requestResolver.resolveEffectiveProfile(ctx, registrySlug, profileSlug)
}
