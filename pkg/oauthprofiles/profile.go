// Package oauthprofiles owns Pinocchio's typed OAuth profile extension.
//
// OAuth protocol mechanics and request-time renewal live in Geppetto. This
// package deliberately owns only Pinocchio's profile schema, secret-state
// validation, and YAML persistence boundaries.
package oauthprofiles

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	geppettoauth "github.com/go-go-golems/geppetto/pkg/steps/ai/credentials/oauth"
)

const (
	// ExtensionKey is the versioned Geppetto profile-extension identifier owned
	// by Pinocchio. OAuth state must not be represented in inference settings.
	ExtensionKey = "pinocchio.oauth@v1"
	// OAuthBearerKind identifies a profile whose OpenAI-compatible provider
	// authorization comes from a renewable OAuth bearer credential.
	OAuthBearerKind = "oauth_bearer"
)

// Profile contains the non-secret OAuth policy and the profile-owned secret
// credential tuple. Do not log or serialize Credential outside the owner-only
// registry file.
type Profile struct {
	AuthorizationURL   string
	TokenURL           string
	ClientID           string
	Scopes             []string
	RefreshTokenPolicy geppettoauth.RefreshTokenPolicy
	Credential         credentials.Credential
}

// IsOAuthProfile reports whether extensions contains the versioned Pinocchio
// OAuth extension. It does not validate the extension.
func IsOAuthProfile(extensions map[string]any) bool {
	if len(extensions) == 0 {
		return false
	}
	_, ok := extensions[ExtensionKey]
	return ok
}

// Parse validates and converts extensions.pinocchio.oauth@v1. A missing OAuth
// extension is reported as (nil, nil). Secret values never appear in errors.
func Parse(extensions map[string]any) (*Profile, error) {
	if !IsOAuthProfile(extensions) {
		return nil, nil
	}
	raw, ok := stringAnyMap(extensions[ExtensionKey])
	if !ok {
		return nil, fmt.Errorf("OAuth profile extension %q must be a mapping", ExtensionKey)
	}

	kind, err := requiredString(raw, "kind")
	if err != nil {
		return nil, err
	}
	if kind != OAuthBearerKind {
		return nil, fmt.Errorf("unsupported OAuth profile kind %q", kind)
	}

	profile := &Profile{RefreshTokenPolicy: geppettoauth.PreservePreviousRefreshToken}
	if profile.AuthorizationURL, err = requiredURL(raw, "authorization_url"); err != nil {
		return nil, err
	}
	if profile.TokenURL, err = requiredURL(raw, "token_url"); err != nil {
		return nil, err
	}
	if profile.ClientID, err = requiredString(raw, "client_id"); err != nil {
		return nil, err
	}
	if value, exists := raw["client_secret"]; exists && strings.TrimSpace(fmt.Sprint(value)) != "" {
		return nil, fmt.Errorf("OAuth profile client_secret is not supported; use a public PKCE client")
	}
	if profile.Scopes, err = optionalStringSlice(raw, "scopes"); err != nil {
		return nil, err
	}
	if policy, exists := raw["refresh_token_policy"]; exists {
		profile.RefreshTokenPolicy, err = parseRefreshTokenPolicy(policy)
		if err != nil {
			return nil, err
		}
	}

	profile.Credential.AccessToken, err = optionalString(raw, "access_token")
	if err != nil {
		return nil, err
	}
	profile.Credential.RefreshToken, err = optionalString(raw, "refresh_token")
	if err != nil {
		return nil, err
	}
	if rawExpiry, exists := raw["expires_at"]; exists {
		expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(fmt.Sprint(rawExpiry)))
		if err != nil {
			return nil, fmt.Errorf("OAuth profile expires_at must be RFC3339")
		}
		profile.Credential.ExpiresAt = expiresAt.UTC()
	}
	return profile, nil
}

// ProtocolConfig creates the reusable Geppetto OAuth client configuration for
// a caller-owned callback URL. The profile format intentionally has no stored
// redirect URL because each browser login binds an exact loopback listener.
func (p *Profile) ProtocolConfig(redirectURL string) (geppettoauth.Config, error) {
	if p == nil {
		return geppettoauth.Config{}, fmt.Errorf("OAuth profile is required")
	}
	if _, err := absoluteHTTPURL(redirectURL, "redirect"); err != nil {
		return geppettoauth.Config{}, err
	}
	return geppettoauth.Config{
		AuthorizationURL: p.AuthorizationURL,
		TokenURL:         p.TokenURL,
		ClientID:         p.ClientID,
		RedirectURL:      redirectURL,
		Scopes:           append([]string(nil), p.Scopes...),
	}, nil
}

// RedactedExtensions clones extensions and replaces OAuth credential fields
// with a marker suitable for inspection output. It never mutates the source.
func RedactedExtensions(extensions map[string]any) map[string]any {
	ret := cloneMap(extensions)
	raw, ok := stringAnyMap(ret[ExtensionKey])
	if !ok {
		return ret
	}
	for _, key := range []string{"access_token", "refresh_token", "client_secret"} {
		if _, exists := raw[key]; exists {
			raw[key] = "<redacted>"
		}
	}
	ret[ExtensionKey] = raw
	return ret
}

func requiredString(raw map[string]any, key string) (string, error) {
	value, err := optionalString(raw, key)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("OAuth profile %s is required", key)
	}
	return value, nil
}

func optionalString(raw map[string]any, key string) (string, error) {
	value, exists := raw[key]
	if !exists || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("OAuth profile %s must be a string", key)
	}
	return strings.TrimSpace(text), nil
}

func optionalStringSlice(raw map[string]any, key string) ([]string, error) {
	value, exists := raw[key]
	if !exists || value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		if strings_, ok := value.([]string); ok {
			items = make([]any, len(strings_))
			for i := range strings_ {
				items[i] = strings_[i]
			}
		} else {
			return nil, fmt.Errorf("OAuth profile %s must be a list of strings", key)
		}
	}
	ret := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for i, item := range items {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("OAuth profile %s[%d] must be a non-empty string", key, i)
		}
		text = strings.TrimSpace(text)
		if _, duplicate := seen[text]; duplicate {
			return nil, fmt.Errorf("OAuth profile %s contains duplicate scope %q", key, text)
		}
		seen[text] = struct{}{}
		ret = append(ret, text)
	}
	return ret, nil
}

func requiredURL(raw map[string]any, key string) (string, error) {
	value, err := requiredString(raw, key)
	if err != nil {
		return "", err
	}
	parsed, err := absoluteHTTPURL(value, key)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func absoluteHTTPURL(raw, key string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("OAuth profile %s must be an absolute HTTP(S) URL", key)
	}
	return parsed, nil
}

func parseRefreshTokenPolicy(value any) (geppettoauth.RefreshTokenPolicy, error) {
	text, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("OAuth profile refresh_token_policy must be a string")
	}
	switch strings.TrimSpace(text) {
	case "", "preserve_previous":
		return geppettoauth.PreservePreviousRefreshToken, nil
	case "require_replacement":
		return geppettoauth.RequireReplacementRefreshToken, nil
	default:
		return 0, fmt.Errorf("OAuth profile refresh_token_policy must be preserve_previous or require_replacement")
	}
}

func stringAnyMap(value any) (map[string]any, bool) {
	map_, ok := value.(map[string]any)
	return map_, ok
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	ret := make(map[string]any, len(in))
	for key, value := range in {
		switch typed := value.(type) {
		case map[string]any:
			ret[key] = cloneMap(typed)
		case []any:
			items := make([]any, len(typed))
			copy(items, typed)
			ret[key] = items
		default:
			ret[key] = value
		}
	}
	return ret
}
