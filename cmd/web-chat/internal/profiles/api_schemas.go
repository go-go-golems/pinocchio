package profiles

import (
	"net/http"
	"sort"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
)

func registerSchemaHandlers(mux *http.ServeMux, opts APIOptions) {
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
