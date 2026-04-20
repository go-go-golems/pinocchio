package main

import (
	"context"
	"net/http"

	appserver "github.com/go-go-golems/pinocchio/cmd/web-chat/app"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/profiles"
	chatapp "github.com/go-go-golems/pinocchio/pkg/evtstream/apps/chat"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

type canonicalRuntimeResolver struct {
	requestResolver *profiles.RequestResolver
	runtimeComposer infruntime.RuntimeBuilder
}

func newCanonicalRuntimeResolver(requestResolver *profiles.RequestResolver, runtimeComposer infruntime.RuntimeBuilder) appserver.RuntimeResolver {
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
	profileSlug, err := r.requestResolver.ResolveProfileSelection(ctx, "", profile, "", "")
	if err != nil {
		return nil, err
	}
	registrySlug, err := r.requestResolver.ResolveRegistrySelection(registry, "", "")
	if err != nil {
		return nil, err
	}
	resolvedProfile, err := r.requestResolver.ResolveEffectiveProfile(ctx, registrySlug, profileSlug)
	if err != nil {
		return nil, err
	}
	plan, err := r.requestResolver.BuildConversationPlan(ctx, "", "", "", resolvedProfile)
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
		ResolvedInferenceSettings:  profiles.CloneResolvedInferenceSettings(plan.Runtime.InferenceSettings),
		ResolvedProfileRuntime:     profiles.ToRuntimeTransport(plan.Runtime),
		ResolvedProfileFingerprint: plan.Runtime.RuntimeFingerprint,
	})
	if err != nil {
		return nil, err
	}
	return &chatapp.ResolvedRuntime{ComposedRuntime: composed}, nil
}
