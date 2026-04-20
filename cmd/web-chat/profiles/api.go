package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

// RegisterAPIHandlers mounts the profile API routes on the provided mux.
func RegisterAPIHandlers(mux *http.ServeMux, profileRegistry gepprofiles.Registry, opts APIOptions) {
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
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
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
			profiles_, err := profileRegistry.ListEngineProfiles(req.Context(), registrySlug)
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
				profiles_, err := profileRegistry.ListEngineProfiles(req.Context(), registrySlug)
				if err != nil {
					writeProfileRegistryError(w, err)
					return
				}
				items = append(items, profileListItemsFromRegistry(registrySlug, registry, profiles_)...)
			}
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].Slug < items[j].Slug
		})
		writeJSONResponse(w, http.StatusOK, items)
	})

	mux.HandleFunc("/api/chat/profiles/", func(w http.ResponseWriter, req *http.Request) {
		slugRaw, action, ok := parseProfilePath(req.URL.Path)
		if !ok {
			http.NotFound(w, req)
			return
		}
		slug, err := gepprofiles.ParseEngineProfileSlug(slugRaw)
		if err != nil {
			http.Error(w, "invalid profile slug", http.StatusBadRequest)
			return
		}

		switch action {
		case "":
			if req.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
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
				profile, err := profileRegistry.GetEngineProfile(req.Context(), registrySlug, slug)
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
				profile, err := profileRegistry.GetEngineProfile(req.Context(), registrySlug, slug)
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
		case "default":
			if req.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, "")
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			profile, err := profileRegistry.GetEngineProfile(req.Context(), registrySlug, slug)
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
			slug := gepprofiles.EngineProfileSlug("")
			registrySlug := opts.DefaultRegistrySlug
			if ck, err := req.Cookie(opts.CurrentProfileCookieName); err == nil && ck != nil {
				if parsedRegistry, parsedProfile, ok := parseCurrentProfileCookieValue(strings.TrimSpace(ck.Value)); ok && profileExists(req.Context(), profileRegistry, parsedRegistry, parsedProfile) {
					registrySlug = parsedRegistry
					slug = parsedProfile
				} else if parsed, err := gepprofiles.ParseEngineProfileSlug(strings.TrimSpace(ck.Value)); err == nil && profileExists(req.Context(), profileRegistry, opts.DefaultRegistrySlug, parsed) {
					slug = parsed
				}
			}
			if slug.IsZero() {
				defaultSlug, err := resolveDefaultEngineProfileSlug(req.Context(), profileRegistry, registrySlug)
				if err != nil {
					http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
					return
				}
				slug = defaultSlug
			}
			writeJSONResponse(w, http.StatusOK, CurrentProfilePayload{
				Profile:  slug.String(),
				Registry: registrySlug.String(),
			})
		case http.MethodPost:
			var body CurrentProfilePayload
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			slugRaw := strings.TrimSpace(body.Profile)
			slug, err := gepprofiles.ParseEngineProfileSlug(slugRaw)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			registrySlug, err := resolveRegistrySlugForAPI(req, opts.DefaultRegistrySlug, body.Registry)
			if err != nil {
				http.Error(w, "invalid registry", http.StatusBadRequest)
				return
			}
			if _, err := profileRegistry.GetEngineProfile(req.Context(), registrySlug, slug); err != nil {
				if errors.Is(err, gepprofiles.ErrProfileNotFound) {
					http.Error(w, "profile not found", http.StatusNotFound)
					return
				}
				http.Error(w, "profile registry unavailable", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     opts.CurrentProfileCookieName,
				Value:    formatCurrentProfileCookieValue(registrySlug, slug),
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Secure:   true,
				HttpOnly: true,
			})
			writeJSONResponse(w, http.StatusOK, CurrentProfilePayload{
				Profile:  slug.String(),
				Registry: registrySlug.String(),
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func formatCurrentProfileCookieValue(registrySlug gepprofiles.RegistrySlug, profileSlug gepprofiles.EngineProfileSlug) string {
	return registrySlug.String() + "/" + profileSlug.String()
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

func profileDocFromModel(registrySlug gepprofiles.RegistrySlug, registry *gepprofiles.EngineProfileRegistry, p *gepprofiles.EngineProfile) ProfileDocument {
	doc := ProfileDocument{Registry: registrySlug.String()}
	if p == nil {
		return doc
	}
	doc.Slug = p.Slug.String()
	doc.DisplayName = p.DisplayName
	doc.Description = p.Description
	if runtime, _, err := infruntime.ProfileRuntimeFromEngineProfile(p); err == nil {
		doc.Runtime = runtime
	}
	doc.Metadata = p.Metadata
	doc.Extensions = cloneExtensionMap(p.Extensions)
	doc.IsDefault = registry != nil && registry.DefaultEngineProfileSlug == p.Slug
	return doc
}

func profileListItemsFromRegistry(registrySlug gepprofiles.RegistrySlug, registry *gepprofiles.EngineProfileRegistry, profiles_ []*gepprofiles.EngineProfile) []ProfileListItem {
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
		defaultPrompt := ""
		if runtime, _, err := infruntime.ProfileRuntimeFromEngineProfile(p); err == nil && runtime != nil {
			defaultPrompt = runtime.SystemPrompt
		}
		items = append(items, ProfileListItem{
			Registry:      registrySlug.String(),
			Slug:          p.Slug.String(),
			DisplayName:   p.DisplayName,
			Description:   p.Description,
			DefaultPrompt: defaultPrompt,
			Extensions:    cloneExtensionMap(p.Extensions),
			IsDefault:     registry != nil && registry.DefaultEngineProfileSlug == p.Slug,
			Version:       p.Metadata.Version,
		})
	}
	return items
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
	_ = definitions
	if codecRegistry != nil {
		for _, codec := range codecRegistry.ListCodecs() {
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
	case errors.Is(err, gepprofiles.ErrReadOnlyStore):
		http.Error(w, err.Error(), http.StatusForbidden)
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

func resolveDefaultEngineProfileSlug(ctx context.Context, profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug) (gepprofiles.EngineProfileSlug, error) {
	registry, err := profileRegistry.GetRegistry(ctx, registrySlug)
	if err != nil {
		return "", err
	}
	if registry != nil && !registry.DefaultEngineProfileSlug.IsZero() {
		return registry.DefaultEngineProfileSlug, nil
	}
	return gepprofiles.MustEngineProfileSlug("default"), nil
}

func profileExists(ctx context.Context, profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug, slug gepprofiles.EngineProfileSlug) bool {
	_, err := profileRegistry.GetEngineProfile(ctx, registrySlug, slug)
	return err == nil
}
