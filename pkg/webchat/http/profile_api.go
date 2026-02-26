package webhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
)

const (
	defaultProfileAPIRegistrySlug = "default"
	defaultCurrentProfileCookie   = "chat_profile"
	defaultProfileWriteActor      = "web-chat"
	defaultProfileWriteSource     = "http-api"
)

type ProfileListItem struct {
	Slug          string         `json:"slug"`
	DisplayName   string         `json:"display_name,omitempty"`
	Description   string         `json:"description,omitempty"`
	DefaultPrompt string         `json:"default_prompt,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
	IsDefault     bool           `json:"is_default,omitempty"`
	Version       uint64         `json:"version,omitempty"`
}

type ProfileDocument struct {
	Registry    string                      `json:"registry"`
	Slug        string                      `json:"slug"`
	DisplayName string                      `json:"display_name,omitempty"`
	Description string                      `json:"description,omitempty"`
	Runtime     gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy      gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata    gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	Extensions  map[string]any              `json:"extensions,omitempty"`
	IsDefault   bool                        `json:"is_default"`
}

type CreateProfileRequest struct {
	Registry        string                       `json:"registry,omitempty"`
	Slug            string                       `json:"slug,omitempty"`
	DisplayName     string                       `json:"display_name,omitempty"`
	Description     string                       `json:"description,omitempty"`
	Runtime         *gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy          *gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata        *gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	Extensions      map[string]any               `json:"extensions,omitempty"`
	SetDefault      bool                         `json:"set_default,omitempty"`
	ExpectedVersion uint64                       `json:"expected_version,omitempty"`
}

type PatchProfileRequest struct {
	Registry        string                       `json:"registry,omitempty"`
	DisplayName     *string                      `json:"display_name,omitempty"`
	Description     *string                      `json:"description,omitempty"`
	Runtime         *gepprofiles.RuntimeSpec     `json:"runtime,omitempty"`
	Policy          *gepprofiles.PolicySpec      `json:"policy,omitempty"`
	Metadata        *gepprofiles.ProfileMetadata `json:"metadata,omitempty"`
	Extensions      *map[string]any              `json:"extensions,omitempty"`
	SetDefault      bool                         `json:"set_default,omitempty"`
	ExpectedVersion uint64                       `json:"expected_version,omitempty"`
}

type SetDefaultProfileRequest struct {
	Registry        string `json:"registry,omitempty"`
	ExpectedVersion uint64 `json:"expected_version,omitempty"`
}

type CurrentProfilePayload struct {
	Slug string `json:"slug"`
}

type MiddlewareSchemaDocument struct {
	Name        string         `json:"name"`
	Version     uint16         `json:"version"`
	DisplayName string         `json:"display_name,omitempty"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema"`
}

type ExtensionSchemaDocument struct {
	Key    string         `json:"key"`
	Schema map[string]any `json:"schema"`
}

type ProfileAPIHandlerOptions struct {
	DefaultRegistrySlug             gepprofiles.RegistrySlug
	EnableCurrentProfileCookieRoute bool
	CurrentProfileCookieName        string
	WriteActor                      string
	WriteSource                     string
	MiddlewareDefinitions           middlewarecfg.DefinitionRegistry
	ExtensionCodecRegistry          gepprofiles.ExtensionCodecRegistry
	ExtensionSchemas                []ExtensionSchemaDocument
}

func (o *ProfileAPIHandlerOptions) normalize() {
	if o.DefaultRegistrySlug.IsZero() {
		o.DefaultRegistrySlug = gepprofiles.MustRegistrySlug(defaultProfileAPIRegistrySlug)
	}
	if strings.TrimSpace(o.CurrentProfileCookieName) == "" {
		o.CurrentProfileCookieName = defaultCurrentProfileCookie
	}
	if strings.TrimSpace(o.WriteActor) == "" {
		o.WriteActor = defaultProfileWriteActor
	}
	if strings.TrimSpace(o.WriteSource) == "" {
		o.WriteSource = defaultProfileWriteSource
	}
	if len(o.ExtensionSchemas) > 0 {
		normalized := make([]ExtensionSchemaDocument, 0, len(o.ExtensionSchemas))
		for _, item := range o.ExtensionSchemas {
			key, err := gepprofiles.ParseExtensionKey(item.Key)
			if err != nil {
				continue
			}
			normalized = append(normalized, ExtensionSchemaDocument{
				Key:    key.String(),
				Schema: cloneExtensionMap(item.Schema),
			})
		}
		sort.Slice(normalized, func(i, j int) bool {
			return normalized[i].Key < normalized[j].Key
		})
		o.ExtensionSchemas = normalized
	}
}

func RegisterProfileAPIHandlers(mux *http.ServeMux, profileRegistry gepprofiles.Registry, opts ProfileAPIHandlerOptions) {
	if mux == nil || profileRegistry == nil {
		return
	}
	opts.normalize()

	mux.HandleFunc("/api/chat/schemas/middlewares", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		items := listMiddlewareSchemas(opts.MiddlewareDefinitions)
		writeJSONResponse(w, http.StatusOK, items)
	})

	mux.HandleFunc("/api/chat/schemas/extensions", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		items := listExtensionSchemas(opts.ExtensionSchemas, opts.MiddlewareDefinitions, opts.ExtensionCodecRegistry)
		writeJSONResponse(w, http.StatusOK, items)
	})

	mux.HandleFunc("/api/chat/profiles", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			registryRaw := ""
			if req != nil {
				registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
			}
			items := make([]ProfileListItem, 0)
			if registryRaw != "" {
				registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				profiles_, err := profileRegistry.ListProfiles(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				items = append(items, profileListItemsFromRegistry(registrySlug, registry, profiles_)...)
			} else {
				registries, err := profileRegistry.ListRegistries(req.Context())
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				for _, summary := range registries {
					registrySlug := summary.Slug
					registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
					if err != nil {
						writeProfileRegistryError(w, err)
						return
					}
					profiles_, err := profileRegistry.ListProfiles(req.Context(), registrySlug)
					if err != nil {
						writeProfileRegistryError(w, err)
						return
					}
					items = append(items, profileListItemsFromRegistry(registrySlug, registry, profiles_)...)
				}
			}
			items = dedupeProfileListItemsBySlug(items)
			sort.Slice(items, func(i, j int) bool {
				return items[i].Slug < items[j].Slug
			})
			writeJSONResponse(w, http.StatusOK, items)
		case http.MethodPost:
			var body CreateProfileRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			normalizedExtensions := cloneExtensionMap(body.Extensions)
			if body.Runtime != nil {
				nextExtensions, err := validateNormalizeAndProjectRuntimeMiddlewaresForWrite(
					body.Runtime,
					normalizedExtensions,
					opts.MiddlewareDefinitions,
				)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				normalizedExtensions = nextExtensions
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, body.Registry)
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			slugRaw := strings.TrimSpace(body.Slug)
			slug, err := gepprofiles.ParseProfileSlug(slugRaw)
			if err != nil {
				http.Error(w, "invalid profile slug", http.StatusBadRequest)
				return
			}
			profile := &gepprofiles.Profile{
				Slug:        slug,
				DisplayName: strings.TrimSpace(body.DisplayName),
				Description: strings.TrimSpace(body.Description),
			}
			if body.Runtime != nil {
				profile.Runtime = *body.Runtime
			}
			if body.Policy != nil {
				profile.Policy = *body.Policy
			}
			if body.Metadata != nil {
				profile.Metadata = *body.Metadata
			}
			if len(normalizedExtensions) > 0 {
				profile.Extensions = normalizedExtensions
			}

			created, err := profileRegistry.CreateProfile(req.Context(), registrySlug, profile, gepprofiles.WriteOptions{
				ExpectedVersion: body.ExpectedVersion,
				Actor:           opts.WriteActor,
				Source:          opts.WriteSource,
			})
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			if body.SetDefault {
				if err := profileRegistry.SetDefaultProfile(req.Context(), registrySlug, created.Slug, gepprofiles.WriteOptions{
					Actor:  opts.WriteActor,
					Source: opts.WriteSource,
				}); err != nil {
					writeProfileRegistryError(w, err)
					return
				}
			}
			registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			writeJSONResponse(w, http.StatusCreated, profileDocFromModel(registrySlug, registry, created))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/chat/profiles/", func(w http.ResponseWriter, req *http.Request) {
		slugRaw, action, ok := parseProfilePath(req.URL.Path)
		if !ok {
			http.NotFound(w, req)
			return
		}
		slug, err := gepprofiles.ParseProfileSlug(slugRaw)
		if err != nil {
			http.Error(w, "invalid profile slug", http.StatusBadRequest)
			return
		}

		switch action {
		case "":
			switch req.Method {
			case http.MethodGet:
				registryRaw := ""
				if req != nil {
					registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
				}
				if registryRaw != "" {
					registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
					if err != nil {
						http.Error(w, "invalid registry", http.StatusBadRequest)
						return
					}
					profile, err := profileRegistry.GetProfile(req.Context(), registrySlug, slug)
					if err != nil {
						writeProfileRegistryError(w, err)
						return
					}
					registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
					if err != nil {
						writeProfileRegistryError(w, err)
						return
					}
					writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
					return
				}

				registries, err := profileRegistry.ListRegistries(req.Context())
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				for _, summary := range registries {
					registrySlug := summary.Slug
					profile, err := profileRegistry.GetProfile(req.Context(), registrySlug, slug)
					if err != nil {
						if errors.Is(err, gepprofiles.ErrProfileNotFound) {
							continue
						}
						writeProfileRegistryError(w, err)
						return
					}
					registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
					if err != nil {
						writeProfileRegistryError(w, err)
						return
					}
					writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
					return
				}
				writeProfileRegistryError(w, gepprofiles.ErrProfileNotFound)
			case http.MethodPatch:
				var body PatchProfileRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, body.Registry)
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				currentProfile, err := profileRegistry.GetProfile(req.Context(), registrySlug, slug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				if body.Runtime != nil || body.Extensions != nil {
					runtimeForValidation := gepprofiles.RuntimeSpec{}
					if currentProfile != nil {
						runtimeForValidation = cloneRuntimeSpec(currentProfile.Runtime)
					}
					if body.Runtime != nil {
						runtimeForValidation = cloneRuntimeSpec(*body.Runtime)
					}
					extensionsForValidation := cloneExtensionMap(nil)
					if currentProfile != nil {
						extensionsForValidation = cloneExtensionMap(currentProfile.Extensions)
					}
					if body.Extensions != nil {
						extensionsForValidation = cloneExtensionMap(*body.Extensions)
					}
					normalizedExtensions, err := validateNormalizeAndProjectRuntimeMiddlewaresForWrite(
						&runtimeForValidation,
						extensionsForValidation,
						opts.MiddlewareDefinitions,
					)
					if err != nil {
						writeProfileRegistryError(w, err)
						return
					}
					if body.Runtime != nil {
						body.Runtime = &runtimeForValidation
					}
					body.Extensions = &normalizedExtensions
				}
				patch := gepprofiles.ProfilePatch{
					DisplayName: body.DisplayName,
					Description: body.Description,
					Runtime:     body.Runtime,
					Policy:      body.Policy,
					Metadata:    body.Metadata,
					Extensions:  body.Extensions,
				}
				profile, err := profileRegistry.UpdateProfile(req.Context(), registrySlug, slug, patch, gepprofiles.WriteOptions{
					ExpectedVersion: body.ExpectedVersion,
					Actor:           opts.WriteActor,
					Source:          opts.WriteSource,
				})
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				if body.SetDefault {
					if err := profileRegistry.SetDefaultProfile(req.Context(), registrySlug, slug, gepprofiles.WriteOptions{
						Actor:  opts.WriteActor,
						Source: opts.WriteSource,
					}); err != nil {
						writeProfileRegistryError(w, err)
						return
					}
				}
				registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
			case http.MethodDelete:
				registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
				if err != nil {
					http.Error(w, "invalid registry", http.StatusBadRequest)
					return
				}
				expectedVersion, err := parseExpectedVersion(req.URL.Query().Get("expected_version"))
				if err != nil {
					http.Error(w, "invalid expected_version", http.StatusBadRequest)
					return
				}
				if err := profileRegistry.DeleteProfile(req.Context(), registrySlug, slug, gepprofiles.WriteOptions{
					ExpectedVersion: expectedVersion,
					Actor:           opts.WriteActor,
					Source:          opts.WriteSource,
				}); err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		case "default":
			if req.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var body SetDefaultProfileRequest
			if req.Body != nil {
				_ = json.NewDecoder(req.Body).Decode(&body)
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, body.Registry)
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			expectedVersion := body.ExpectedVersion
			if expectedVersion == 0 {
				expectedVersion, err = parseExpectedVersion(req.URL.Query().Get("expected_version"))
				if err != nil {
					http.Error(w, "invalid expected_version", http.StatusBadRequest)
					return
				}
			}
			if err := profileRegistry.SetDefaultProfile(req.Context(), registrySlug, slug, gepprofiles.WriteOptions{
				ExpectedVersion: expectedVersion,
				Actor:           opts.WriteActor,
				Source:          opts.WriteSource,
			}); err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			profile, err := profileRegistry.GetProfile(req.Context(), registrySlug, slug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			registry, err := profileRegistry.GetRegistry(req.Context(), registrySlug)
			if err != nil {
				writeProfileRegistryError(w, err)
				return
			}
			writeJSONResponse(w, http.StatusOK, profileDocFromModel(registrySlug, registry, profile))
		default:
			http.NotFound(w, req)
		}
	})

	if !opts.EnableCurrentProfileCookieRoute {
		return
	}

	mux.HandleFunc("/api/chat/profile", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			slug := gepprofiles.ProfileSlug("")
			if ck, err := req.Cookie(opts.CurrentProfileCookieName); err == nil && ck != nil {
				if parsed, err := gepprofiles.ParseProfileSlug(strings.TrimSpace(ck.Value)); err == nil && profileExists(req.Context(), profileRegistry, opts.DefaultRegistrySlug, parsed) {
					slug = parsed
				}
			}
			if slug.IsZero() {
				defaultSlug, err := resolveDefaultProfileSlug(req.Context(), profileRegistry, opts.DefaultRegistrySlug)
				if err != nil {
					http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
					return
				}
				slug = defaultSlug
			}
			writeJSONResponse(w, http.StatusOK, CurrentProfilePayload{Slug: slug.String()})
		case http.MethodPost:
			var body CurrentProfilePayload
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			slugRaw := strings.TrimSpace(body.Slug)
			slug, err := gepprofiles.ParseProfileSlug(slugRaw)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if _, err := profileRegistry.GetProfile(req.Context(), opts.DefaultRegistrySlug, slug); err != nil {
				if errors.Is(err, gepprofiles.ErrProfileNotFound) {
					http.Error(w, "profile not found", http.StatusNotFound)
					return
				}
				http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     opts.CurrentProfileCookieName,
				Value:    slug.String(),
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Secure:   true,
				HttpOnly: true,
			})
			writeJSONResponse(w, http.StatusOK, CurrentProfilePayload{Slug: slug.String()})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func parseProfilePath(path string) (string, string, bool) {
	const prefix = "/api/chat/profiles/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) > 2 {
		return "", "", false
	}
	slug := strings.TrimSpace(parts[0])
	if slug == "" {
		return "", "", false
	}
	action := ""
	if len(parts) == 2 {
		action = strings.TrimSpace(parts[1])
	}
	return slug, action, true
}

func parseExpectedVersion(raw string) (uint64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	v, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expected_version: %w", err)
	}
	return v, nil
}

func resolveRegistrySlugForAPI(req *http.Request, defaultSlug gepprofiles.RegistrySlug, bodyRegistryRaw string) (gepprofiles.RegistrySlug, error) {
	registryRaw := strings.TrimSpace(bodyRegistryRaw)
	if registryRaw == "" && req != nil {
		registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
	}
	if registryRaw == "" {
		return defaultSlug, nil
	}
	registrySlug, err := gepprofiles.ParseRegistrySlug(registryRaw)
	if err != nil {
		return "", err
	}
	return registrySlug, nil
}

func profileDocFromModel(registrySlug gepprofiles.RegistrySlug, registry *gepprofiles.ProfileRegistry, p *gepprofiles.Profile) ProfileDocument {
	doc := ProfileDocument{Registry: registrySlug.String()}
	if p == nil {
		return doc
	}
	doc.Slug = p.Slug.String()
	doc.DisplayName = p.DisplayName
	doc.Description = p.Description
	doc.Runtime = p.Runtime
	doc.Policy = p.Policy
	doc.Metadata = p.Metadata
	doc.Extensions = cloneExtensionMap(p.Extensions)
	doc.IsDefault = registry != nil && registry.DefaultProfileSlug == p.Slug
	return doc
}

func profileListItemsFromRegistry(registrySlug gepprofiles.RegistrySlug, registry *gepprofiles.ProfileRegistry, profiles_ []*gepprofiles.Profile) []ProfileListItem {
	sort.Slice(profiles_, func(i, j int) bool {
		if profiles_[i] == nil {
			return false
		}
		if profiles_[j] == nil {
			return true
		}
		return profiles_[i].Slug < profiles_[j].Slug
	})

	items := make([]ProfileListItem, 0, len(profiles_))
	for _, p := range profiles_ {
		if p == nil {
			continue
		}
		items = append(items, ProfileListItem{
			Slug:          p.Slug.String(),
			DisplayName:   p.DisplayName,
			Description:   p.Description,
			DefaultPrompt: p.Runtime.SystemPrompt,
			Extensions:    cloneExtensionMap(p.Extensions),
			IsDefault:     registry != nil && registry.DefaultProfileSlug == p.Slug,
			Version:       p.Metadata.Version,
		})
	}
	return items
}

func dedupeProfileListItemsBySlug(items []ProfileListItem) []ProfileListItem {
	if len(items) <= 1 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]ProfileListItem, 0, len(items))
	for _, item := range items {
		slug := strings.TrimSpace(item.Slug)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, item)
	}
	return out
}

func cloneExtensionMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func listMiddlewareSchemas(definitions middlewarecfg.DefinitionRegistry) []MiddlewareSchemaDocument {
	if definitions == nil {
		return []MiddlewareSchemaDocument{}
	}
	defs := definitions.ListDefinitions()
	items := make([]MiddlewareSchemaDocument, 0, len(defs))
	for _, def := range defs {
		if def == nil {
			continue
		}
		name := strings.TrimSpace(def.Name())
		if name == "" {
			continue
		}
		version, displayName, description := middlewareSchemaMetadata(def)
		items = append(items, MiddlewareSchemaDocument{
			Name:        name,
			Version:     version,
			DisplayName: displayName,
			Description: description,
			Schema:      cloneExtensionMap(def.ConfigJSONSchema()),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

type middlewareVersionProvider interface {
	MiddlewareVersion() uint16
}

type middlewareDisplayMetadataProvider interface {
	MiddlewareDisplayName() string
	MiddlewareDescription() string
}

func middlewareSchemaMetadata(def middlewarecfg.Definition) (uint16, string, string) {
	if def == nil {
		return 1, "", ""
	}
	version := uint16(1)
	displayName := ""
	description := ""

	if provider, ok := def.(middlewareVersionProvider); ok {
		if v := provider.MiddlewareVersion(); v > 0 {
			version = v
		}
	}
	if provider, ok := def.(middlewareDisplayMetadataProvider); ok {
		displayName = strings.TrimSpace(provider.MiddlewareDisplayName())
		description = strings.TrimSpace(provider.MiddlewareDescription())
	}

	schema := def.ConfigJSONSchema()
	if displayName == "" {
		if raw, ok := schema["title"].(string); ok {
			displayName = strings.TrimSpace(raw)
		}
	}
	if description == "" {
		if raw, ok := schema["description"].(string); ok {
			description = strings.TrimSpace(raw)
		}
	}
	if displayName == "" {
		displayName = strings.TrimSpace(def.Name())
	}
	return version, displayName, description
}

func listExtensionSchemas(
	explicit []ExtensionSchemaDocument,
	definitions middlewarecfg.DefinitionRegistry,
	codecRegistry gepprofiles.ExtensionCodecRegistry,
) []ExtensionSchemaDocument {
	byKey := map[string]ExtensionSchemaDocument{}
	for _, item := range explicit {
		key, err := gepprofiles.ParseExtensionKey(item.Key)
		if err != nil {
			continue
		}
		byKey[key.String()] = ExtensionSchemaDocument{
			Key:    key.String(),
			Schema: cloneExtensionMap(item.Schema),
		}
	}
	if definitions != nil {
		for _, def := range definitions.ListDefinitions() {
			if def == nil {
				continue
			}
			key, err := gepprofiles.MiddlewareConfigExtensionKey(def.Name())
			if err != nil {
				continue
			}
			keyString := key.String()
			if _, exists := byKey[keyString]; exists {
				continue
			}
			byKey[keyString] = ExtensionSchemaDocument{
				Key:    keyString,
				Schema: middlewareConfigExtensionSchema(def.ConfigJSONSchema()),
			}
		}
	}
	if lister, ok := codecRegistry.(gepprofiles.ExtensionCodecLister); ok {
		for _, codec := range lister.ListCodecs() {
			if codec == nil {
				continue
			}
			key := codec.Key()
			if key.IsZero() {
				continue
			}
			keyString := key.String()
			if _, exists := byKey[keyString]; exists {
				continue
			}
			schemaCodec, ok := codec.(gepprofiles.ExtensionSchemaCodec)
			if !ok {
				continue
			}
			schema := cloneExtensionMap(schemaCodec.JSONSchema())
			if len(schema) == 0 {
				continue
			}
			byKey[keyString] = ExtensionSchemaDocument{
				Key:    keyString,
				Schema: schema,
			}
		}
	}
	items := make([]ExtensionSchemaDocument, 0, len(byKey))
	for _, item := range byKey {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

func middlewareConfigExtensionSchema(configSchema map[string]any) map[string]any {
	typedConfigSchema := cloneExtensionMap(configSchema)
	if typedConfigSchema == nil {
		typedConfigSchema = map[string]any{"type": "object"}
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"instances": map[string]any{
				"type":                 "object",
				"additionalProperties": typedConfigSchema,
			},
		},
		"required":             []any{"instances"},
		"additionalProperties": false,
	}
}

func validateNormalizeAndProjectRuntimeMiddlewaresForWrite(
	runtime *gepprofiles.RuntimeSpec,
	extensions map[string]any,
	definitions middlewarecfg.DefinitionRegistry,
) (map[string]any, error) {
	normalizedExtensions, err := gepprofiles.ProjectRuntimeMiddlewareConfigsToExtensions(runtime, extensions)
	if err != nil {
		return nil, &gepprofiles.ValidationError{
			Field:  "runtime.middlewares.config",
			Reason: err.Error(),
		}
	}
	if runtime == nil || len(runtime.Middlewares) == 0 || definitions == nil {
		return normalizedExtensions, nil
	}
	for i, middlewareUse := range runtime.Middlewares {
		name := strings.TrimSpace(middlewareUse.Name)
		if name == "" {
			continue
		}
		fieldPrefix := fmt.Sprintf("runtime.middlewares[%d]", i)

		def, ok := definitions.GetDefinition(name)
		if !ok {
			return nil, &gepprofiles.ValidationError{
				Field:  fieldPrefix + ".name",
				Reason: fmt.Sprintf("unknown middleware %q", name),
			}
		}

		payload, _, err := gepprofiles.MiddlewareConfigFromExtensions(
			normalizedExtensions,
			gepprofiles.MiddlewareUse{
				Name:    name,
				ID:      strings.TrimSpace(middlewareUse.ID),
				Enabled: cloneBoolPtr(middlewareUse.Enabled),
			},
			i,
		)
		if err != nil {
			return nil, &gepprofiles.ValidationError{
				Field:  fieldPrefix + ".config",
				Reason: err.Error(),
			}
		}
		sources := make([]middlewarecfg.Source, 0, 1)
		if len(payload) > 0 {
			sources = append(sources, profileWritePayloadSource{
				payload: payload,
			})
		}
		resolver := middlewarecfg.NewResolver(sources...)
		resolvedConfig, err := resolver.Resolve(def, gepprofiles.MiddlewareUse{
			Name:    name,
			ID:      strings.TrimSpace(middlewareUse.ID),
			Enabled: cloneBoolPtr(middlewareUse.Enabled),
		})
		if err != nil {
			return nil, &gepprofiles.ValidationError{
				Field:  fieldPrefix + ".config",
				Reason: err.Error(),
			}
		}
		normalizedExtensions, err = gepprofiles.SetMiddlewareConfigInExtensions(
			normalizedExtensions,
			gepprofiles.MiddlewareUse{
				Name:    name,
				ID:      strings.TrimSpace(middlewareUse.ID),
				Enabled: cloneBoolPtr(middlewareUse.Enabled),
			},
			i,
			cloneExtensionMap(resolvedConfig.Config),
		)
		if err != nil {
			return nil, &gepprofiles.ValidationError{
				Field:  fieldPrefix + ".config",
				Reason: err.Error(),
			}
		}

		runtime.Middlewares[i].Name = name
		runtime.Middlewares[i].ID = strings.TrimSpace(middlewareUse.ID)
		runtime.Middlewares[i].Enabled = cloneBoolPtr(middlewareUse.Enabled)
		runtime.Middlewares[i].Config = nil
	}
	return normalizedExtensions, nil
}

type profileWritePayloadSource struct {
	payload map[string]any
}

func (s profileWritePayloadSource) Name() string {
	return "profile_write"
}

func (s profileWritePayloadSource) Layer() middlewarecfg.SourceLayer {
	return middlewarecfg.SourceLayerProfile
}

func (s profileWritePayloadSource) Payload(middlewarecfg.Definition, gepprofiles.MiddlewareUse) (map[string]any, bool, error) {
	if len(s.payload) == 0 {
		return nil, false, nil
	}
	return cloneExtensionMap(s.payload), true, nil
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func cloneRuntimeSpec(in gepprofiles.RuntimeSpec) gepprofiles.RuntimeSpec {
	out := gepprofiles.RuntimeSpec{
		StepSettingsPatch: cloneExtensionMap(in.StepSettingsPatch),
		SystemPrompt:      in.SystemPrompt,
		Middlewares:       append([]gepprofiles.MiddlewareUse(nil), in.Middlewares...),
		Tools:             append([]string(nil), in.Tools...),
	}
	for i := range out.Middlewares {
		out.Middlewares[i].Name = strings.TrimSpace(out.Middlewares[i].Name)
		out.Middlewares[i].ID = strings.TrimSpace(out.Middlewares[i].ID)
		out.Middlewares[i].Enabled = cloneBoolPtr(out.Middlewares[i].Enabled)
		out.Middlewares[i].Config = deepCopyAny(out.Middlewares[i].Config)
	}
	return out
}

func deepCopyAny(in any) any {
	switch v := in.(type) {
	case map[string]any:
		return cloneExtensionMap(v)
	case []any:
		ret := make([]any, 0, len(v))
		for _, item := range v {
			ret = append(ret, deepCopyAny(item))
		}
		return ret
	default:
		return v
	}
}

func writeProfileRegistryError(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, gepprofiles.ErrProfileNotFound):
		http.Error(w, "profile not found", http.StatusNotFound)
	case errors.Is(err, gepprofiles.ErrRegistryNotFound):
		http.Error(w, "registry not found", http.StatusNotFound)
	case errors.Is(err, gepprofiles.ErrValidation):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, gepprofiles.ErrPolicyViolation):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, gepprofiles.ErrReadOnlyStore):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, gepprofiles.ErrVersionConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
	}
}

func writeJSONResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if status > 0 {
		w.WriteHeader(status)
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func resolveDefaultProfileSlug(ctx context.Context, profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug) (gepprofiles.ProfileSlug, error) {
	registry, err := profileRegistry.GetRegistry(ctx, registrySlug)
	if err != nil {
		return "", err
	}
	if registry != nil && !registry.DefaultProfileSlug.IsZero() {
		return registry.DefaultProfileSlug, nil
	}
	return gepprofiles.MustProfileSlug("default"), nil
}

func profileExists(ctx context.Context, profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug, slug gepprofiles.ProfileSlug) bool {
	_, err := profileRegistry.GetProfile(ctx, registrySlug, slug)
	return err == nil
}
