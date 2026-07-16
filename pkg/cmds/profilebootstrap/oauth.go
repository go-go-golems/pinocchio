package profilebootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	geppettoauth "github.com/go-go-golems/geppetto/pkg/steps/ai/credentials/oauth"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	"github.com/go-go-golems/pinocchio/pkg/oauthprofiles"
)

const runtimeOAuthRedirectURL = "http://127.0.0.1/oauth/callback"

// ResolvedOAuthProfile binds one resolved profile to the sole direct YAML
// registry file that owns its secret tuple and its exact outbound request.
type ResolvedOAuthProfile struct {
	Profile *oauthprofiles.Profile
	Store   *oauthprofiles.YAMLStore
	Request credentials.Request
}

// ResolveOAuthProfile resolves the selected profile's OAuth extension. OAuth
// profiles are deliberately supported only from one direct YAML registry file;
// inline, composed, SQLite, and remote-like sources have no safe write target.
func ResolveOAuthProfile(ctx context.Context, resolved *ResolvedCLIEngineSettings) (*ResolvedOAuthProfile, error) {
	if resolved == nil || resolved.ResolvedEngineProfile == nil {
		return nil, nil
	}
	if resolved.ProfileRuntime == nil || resolved.ProfileRuntime.Reader() == nil {
		return nil, errors.New("OAuth profile resolution requires a profile registry runtime")
	}
	if resolved.FinalInferenceSettings == nil {
		return nil, errors.New("OAuth profile resolution requires final inference settings")
	}

	profile, err := resolved.ProfileRuntime.Reader().GetEngineProfile(ctx, resolved.ResolvedEngineProfile.RegistrySlug, resolved.ResolvedEngineProfile.EngineProfileSlug)
	if err != nil {
		return nil, fmt.Errorf("load selected OAuth profile: %w", err)
	}
	oauthProfile, err := oauthprofiles.Parse(profile.Extensions)
	if err != nil {
		return nil, err
	}
	if oauthProfile == nil {
		return nil, nil
	}

	request, err := oauthCredentialRequest(resolved.FinalInferenceSettings)
	if err != nil {
		return nil, err
	}
	if err := rejectStaticOAuthCredential(resolved.FinalInferenceSettings, request); err != nil {
		return nil, err
	}
	path, err := directYAMLRegistryPath(resolved.ProfileRuntime.ProfileSettings.ProfileRegistries, resolved.ResolvedEngineProfile.RegistrySlug, resolved.ResolvedEngineProfile.EngineProfileSlug)
	if err != nil {
		return nil, err
	}
	store, err := oauthprofiles.NewYAMLStore(path, resolved.ResolvedEngineProfile.RegistrySlug, resolved.ResolvedEngineProfile.EngineProfileSlug, request)
	if err != nil {
		return nil, err
	}
	return &ResolvedOAuthProfile{Profile: oauthProfile, Store: store, Request: request}, nil
}

// NewOAuthClient creates a profile-bound reusable protocol client for the
// caller's exact callback URL. Runtime renewal uses a loopback placeholder
// because refresh grants never use the redirect URL; browser login passes the
// bound listener URL instead.
func (r *ResolvedOAuthProfile) NewOAuthClient(redirectURL string) (*geppettoauth.Client, error) {
	if r == nil || r.Profile == nil {
		return nil, errors.New("resolved OAuth profile is required")
	}
	config, err := r.Profile.ProtocolConfig(redirectURL)
	if err != nil {
		return nil, err
	}
	return geppettoauth.NewClient(config, geppettoauth.WithRefreshTokenPolicy(r.Profile.RefreshTokenPolicy))
}

// NewBearerTokenSource constructs the Geppetto renewable source for this
// profile. The store persists a rotated tuple before the source caches it.
func (r *ResolvedOAuthProfile) NewBearerTokenSource() (credentials.BearerTokenSource, error) {
	client, err := r.NewOAuthClient(runtimeOAuthRedirectURL)
	if err != nil {
		return nil, err
	}
	refresher, err := oauthprofiles.NewRefresher(client)
	if err != nil {
		return nil, err
	}
	return credentials.NewRenewableBearerTokenSource(r.Store, refresher)
}

// NewBearerTokenSourceForResolvedSettings returns the selected profile's
// host-owned renewable source. Static-key profiles return nil. Callers must
// keep this Go interface out of inference settings and JavaScript values.
func NewBearerTokenSourceForResolvedSettings(ctx context.Context, resolved *ResolvedCLIEngineSettings) (credentials.BearerTokenSource, error) {
	oauthProfile, err := ResolveOAuthProfile(ctx, resolved)
	if err != nil || oauthProfile == nil {
		return nil, err
	}
	return oauthProfile.NewBearerTokenSource()
}

// NewEngineFactoryForResolvedSettings returns a standard factory with a
// renewable bearer source only when the selected profile explicitly opts into
// OAuth. Static-key profiles retain existing behavior.
func NewEngineFactoryForResolvedSettings(ctx context.Context, resolved *ResolvedCLIEngineSettings) (factory.EngineFactory, error) {
	source, err := NewBearerTokenSourceForResolvedSettings(ctx, resolved)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return factory.NewStandardEngineFactory(), nil
	}
	return factory.NewStandardEngineFactory(factory.WithBearerTokenSource(source)), nil
}

func directYAMLRegistryPath(entries []string, registrySlug gepprofiles.RegistrySlug, profileSlug gepprofiles.EngineProfileSlug) (string, error) {
	specs, err := gepprofiles.ParseRegistrySourceSpecs(entries)
	if err != nil {
		return "", err
	}
	matches := []string{}
	for _, spec := range specs {
		if spec.Kind != gepprofiles.RegistrySourceKindYAML {
			continue
		}
		data, err := os.ReadFile(spec.Path)
		if err != nil {
			return "", fmt.Errorf("read OAuth profile registry source: %w", err)
		}
		registry, err := gepprofiles.DecodeEngineProfileYAMLSingleRegistry(data)
		if err != nil {
			return "", fmt.Errorf("decode OAuth profile registry source: %w", err)
		}
		if registry == nil || registry.Slug != registrySlug {
			continue
		}
		profile := registry.Profiles[profileSlug]
		if profile == nil || !oauthprofiles.IsOAuthProfile(profile.Extensions) {
			continue
		}
		path, err := filepath.Abs(spec.Path)
		if err != nil {
			return "", fmt.Errorf("resolve OAuth profile registry path: %w", err)
		}
		matches = append(matches, path)
	}
	switch len(matches) {
	case 0:
		return "", errors.New("OAuth profiles require an explicit direct YAML profile registry source")
	case 1:
		return matches[0], nil
	default:
		return "", errors.New("OAuth profile is present in multiple direct YAML registry sources")
	}
}

func oauthCredentialRequest(settings *aisettings.InferenceSettings) (credentials.Request, error) {
	if settings == nil || settings.Chat == nil || settings.Chat.ApiType == nil || settings.API == nil {
		return credentials.Request{}, errors.New("OAuth profile requires chat API type and API settings")
	}
	apiType := strings.ToLower(strings.TrimSpace(string(*settings.Chat.ApiType)))
	switch apiType {
	case string(types.ApiTypeOpenAI), string(types.ApiTypeAnyScale), string(types.ApiTypeFireworks):
		baseURL := strings.TrimSpace(settings.API.BaseUrls[apiType+"-base-url"])
		if baseURL == "" {
			return credentials.Request{}, fmt.Errorf("OAuth profile requires %s-base-url", apiType)
		}
		return credentials.Request{Provider: apiType, BaseURL: baseURL}, nil
	case string(types.ApiTypeOpenResponses), string(types.ApiTypeOpenAIResponses):
		baseURL := "https://api.openai.com/v1"
		for _, key := range []string{
			string(types.ApiTypeOpenResponses) + "-base-url",
			string(types.ApiTypeOpenAIResponses) + "-base-url",
			string(types.ApiTypeOpenAI) + "-base-url",
		} {
			if value := strings.TrimSpace(settings.API.BaseUrls[key]); value != "" {
				baseURL = value
				break
			}
		}
		return credentials.Request{Provider: string(types.ApiTypeOpenResponses), BaseURL: baseURL}, nil
	case string(types.ApiTypeClaude), "anthropic":
		baseURL := strings.TrimSpace(settings.API.BaseUrls[string(types.ApiTypeClaude)+"-base-url"])
		if baseURL == "" {
			return credentials.Request{}, errors.New("OAuth profile requires claude-base-url")
		}
		return credentials.Request{Provider: string(types.ApiTypeClaude), BaseURL: baseURL}, nil
	default:
		return credentials.Request{}, fmt.Errorf("OAuth profile does not support chat API type %q", apiType)
	}
}

func rejectStaticOAuthCredential(settings *aisettings.InferenceSettings, request credentials.Request) error {
	if settings == nil || settings.API == nil {
		return errors.New("OAuth profile requires API settings")
	}
	keys := []string{request.Provider + "-api-key"}
	if request.Provider == string(types.ApiTypeOpenResponses) {
		keys = []string{
			string(types.ApiTypeOpenResponses) + "-api-key",
			string(types.ApiTypeOpenAIResponses) + "-api-key",
			string(types.ApiTypeOpenAI) + "-api-key",
		}
	}
	for _, key := range keys {
		if strings.TrimSpace(settings.API.APIKeys[key]) != "" {
			return errors.New("OAuth profile cannot also configure a static provider API key")
		}
	}
	return nil
}
