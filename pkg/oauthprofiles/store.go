package oauthprofiles

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
)

// YAMLStore persists one OAuth profile's credential tuple in one direct
// Geppetto YAML registry file. It is intentionally not usable with composed,
// inline, SQLite, or remote profile sources because a refresh must have one
// auditable owner-only target file.
type YAMLStore struct {
	path     string
	registry gepprofiles.RegistrySlug
	profile  gepprofiles.EngineProfileSlug
	expected credentials.Request
}

var _ credentials.Store = (*YAMLStore)(nil)

// NewYAMLStore creates a store bound to one registry/profile and exactly one
// outbound provider/base URL pair.
func NewYAMLStore(path string, registry gepprofiles.RegistrySlug, profile gepprofiles.EngineProfileSlug, expected credentials.Request) (*YAMLStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("OAuth profile registry path is required")
	}
	if registry.IsZero() || profile.IsZero() {
		return nil, errors.New("OAuth profile registry and profile slugs are required")
	}
	if _, err := requestKey(expected); err != nil {
		return nil, err
	}
	return &YAMLStore{
		path:     filepath.Clean(path),
		registry: registry,
		profile:  profile,
		expected: normalizeRequest(expected),
	}, nil
}

// Load returns the current persisted credential after checking file security,
// the target profile identity, and the request identity. Errors never include
// OAuth token contents.
func (s *YAMLStore) Load(ctx context.Context, request credentials.Request) (credentials.Credential, error) {
	if err := s.validateRequest(request); err != nil {
		return credentials.Credential{}, err
	}
	if err := contextErr(ctx); err != nil {
		return credentials.Credential{}, err
	}
	var credential credentials.Credential
	err := withRegistryLock(s.path, false, func() error {
		profile, err := s.loadProfile()
		if err != nil {
			return err
		}
		parsed, err := Parse(profile.Extensions)
		if err != nil {
			return err
		}
		if parsed == nil {
			return errors.New("OAuth profile extension is missing")
		}
		credential = parsed.Credential
		return nil
	})
	if err != nil {
		return credentials.Credential{}, err
	}
	return credential, nil
}

// Save atomically replaces access, refresh, and expiry state as one tuple. A
// refresh token is always required: Geppetto's refresh policy has already
// chosen whether an omitted provider token was preserved or rejected.
func (s *YAMLStore) Save(ctx context.Context, request credentials.Request, credential credentials.Credential) error {
	if err := s.validateRequest(request); err != nil {
		return err
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(credential.AccessToken) == "" {
		return errors.New("OAuth credential access token is required")
	}
	if strings.TrimSpace(credential.RefreshToken) == "" {
		return errors.New("OAuth credential refresh token is required")
	}

	return withRegistryLock(s.path, true, func() error {
		registry, profile, err := s.loadRegistryAndProfile()
		if err != nil {
			return err
		}
		parsed, err := Parse(profile.Extensions)
		if err != nil {
			return err
		}
		if parsed == nil {
			return errors.New("OAuth profile extension is missing")
		}
		setCredential(profile.Extensions, credential)
		registry.Profiles[s.profile] = profile
		data, err := gepprofiles.EncodeEngineProfileYAMLSingleRegistry(registry)
		if err != nil {
			return fmt.Errorf("encode OAuth profile registry: %w", err)
		}
		return atomicWriteOwnerOnly(s.path, data)
	})
}

// Path returns the direct YAML registry path, for diagnostics that must never
// include credential values.
func (s *YAMLStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *YAMLStore) validateRequest(request credentials.Request) error {
	if s == nil {
		return errors.New("nil OAuth profile credential store")
	}
	if _, err := requestKey(request); err != nil {
		return err
	}
	if normalizeRequest(request) != s.expected {
		return errors.New("OAuth credential request does not match the selected profile")
	}
	return nil
}

func (s *YAMLStore) loadProfile() (*gepprofiles.EngineProfile, error) {
	_, profile, err := s.loadRegistryAndProfile()
	return profile, err
}

func (s *YAMLStore) loadRegistryAndProfile() (*gepprofiles.EngineProfileRegistry, *gepprofiles.EngineProfile, error) {
	if err := ensureOwnerOnlyRegistry(s.path); err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, nil, fmt.Errorf("read OAuth profile registry: %w", err)
	}
	registry, err := gepprofiles.DecodeEngineProfileYAMLSingleRegistry(data)
	if err != nil {
		return nil, nil, fmt.Errorf("decode OAuth profile registry: %w", err)
	}
	if registry == nil || registry.Slug != s.registry {
		return nil, nil, errors.New("OAuth profile registry does not match selected registry")
	}
	profile := registry.Profiles[s.profile]
	if profile == nil {
		return nil, nil, errors.New("OAuth profile does not exist in selected registry")
	}
	return registry, profile, nil
}

func setCredential(extensions map[string]any, credential credentials.Credential) {
	oauth, _ := stringAnyMap(extensions[ExtensionKey])
	if oauth == nil {
		oauth = map[string]any{}
	}
	oauth["access_token"] = credential.AccessToken
	oauth["refresh_token"] = credential.RefreshToken
	if credential.ExpiresAt.IsZero() {
		delete(oauth, "expires_at")
	} else {
		oauth["expires_at"] = credential.ExpiresAt.UTC().Format(time.RFC3339)
	}
	extensions[ExtensionKey] = oauth
}

func ensureOwnerOnlyRegistry(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat OAuth profile registry: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("OAuth profile registry must be a regular file")
	}
	if info.Mode().Perm() != 0o600 {
		return errors.New("OAuth profile registry must have mode 0600")
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("stat OAuth profile registry directory: %w", err)
	}
	if !dirInfo.IsDir() || dirInfo.Mode().Perm()&0o022 != 0 {
		return errors.New("OAuth profile registry directory must not be group or world writable")
	}
	return nil
}

func atomicWriteOwnerOnly(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".oauth-")
	if err != nil {
		return fmt.Errorf("create OAuth profile temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	completed := false
	defer func() {
		if !completed {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set OAuth profile temporary file mode: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write OAuth profile temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync OAuth profile temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close OAuth profile temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace OAuth profile registry: %w", err)
	}
	if err := syncDirectory(dir); err != nil {
		return fmt.Errorf("sync OAuth profile registry directory: %w", err)
	}
	completed = true
	return nil
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func normalizeRequest(request credentials.Request) credentials.Request {
	request.Provider = strings.ToLower(strings.TrimSpace(request.Provider))
	request.BaseURL = canonicalBaseURL(request.BaseURL)
	return request
}

func requestKey(request credentials.Request) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		return "", errors.New("OAuth credential request provider is required")
	}
	baseURL := canonicalBaseURL(request.BaseURL)
	if baseURL == "" {
		return "", errors.New("OAuth credential request base URL is required")
	}
	return provider + "\x00" + baseURL, nil
}

func canonicalBaseURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(strings.TrimSpace(raw), "/")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}

// Platform-specific files provide locking and directory fsync while this file
// keeps credential persistence logic testable.
