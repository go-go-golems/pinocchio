package profiles

import (
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

// RequestResolutionError is a typed error allowing handlers to choose an HTTP status code
// without duplicating policy logic.
type RequestResolutionError struct {
	Status    int
	ClientMsg string
	Err       error
}

func (e *RequestResolutionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.ClientMsg + ": " + e.Err.Error()
	}
	return e.ClientMsg
}

func (e *RequestResolutionError) Unwrap() error { return e.Err }

// ConversationPlan captures the resolved plan for a conversation request.
type ConversationPlan struct {
	ConvID         string
	Prompt         string
	IdempotencyKey string
	Runtime        *ResolvedRuntime
}

// ResolvedRuntime is the app-local runtime description produced by profile resolution.
type ResolvedRuntime struct {
	SystemPrompt       string
	Middlewares        []infruntime.MiddlewareUse
	ToolNames          []string
	RuntimeKey         string
	RuntimeFingerprint string
	ProfileVersion     uint64
	InferenceSettings  *aisettings.InferenceSettings
	ProfileMetadata    map[string]any
}

// ProfileListItem is the JSON shape for profile listing.
type ProfileListItem struct {
	Registry      string         `json:"registry"`
	Slug          string         `json:"slug"`
	DisplayName   string         `json:"display_name,omitempty"`
	Description   string         `json:"description,omitempty"`
	DefaultPrompt string         `json:"default_prompt,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
	IsDefault     bool           `json:"is_default,omitempty"`
	Version       uint64         `json:"version,omitempty"`
}

// ProfileDocument is the JSON shape for a single profile detail response.
type ProfileDocument struct {
	Registry    string                            `json:"registry"`
	Slug        string                            `json:"slug"`
	DisplayName string                            `json:"display_name,omitempty"`
	Description string                            `json:"description,omitempty"`
	Runtime     *infruntime.ProfileRuntime        `json:"runtime,omitempty"`
	Metadata    gepprofiles.EngineProfileMetadata `json:"metadata,omitempty"`
	Extensions  map[string]any                    `json:"extensions,omitempty"`
	IsDefault   bool                              `json:"is_default"`
}

// CurrentProfilePayload is the request/response shape for the current profile cookie route.
type CurrentProfilePayload struct {
	Profile  string `json:"profile"`
	Registry string `json:"registry,omitempty"`
}

// MiddlewareSchemaDocument is the JSON shape for middleware schema listing.
type MiddlewareSchemaDocument struct {
	Name        string         `json:"name"`
	Version     uint16         `json:"version"`
	DisplayName string         `json:"display_name,omitempty"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema"`
}

// ExtensionSchemaDocument is the JSON shape for extension schema listing.
type ExtensionSchemaDocument struct {
	Key    string         `json:"key"`
	Schema map[string]any `json:"schema"`
}

// APIOptions configures the profile API handlers.
type APIOptions struct {
	DefaultRegistrySlug             gepprofiles.RegistrySlug
	EnableCurrentProfileCookieRoute bool
	CurrentProfileCookieName        string
	MiddlewareDefinitions           middlewarecfg.DefinitionRegistry
	ExtensionCodecRegistry          gepprofiles.ExtensionCodecRegistry
	ExtensionSchemas                []ExtensionSchemaDocument
}

func (o *APIOptions) normalize() {
	if o.DefaultRegistrySlug.IsZero() {
		o.DefaultRegistrySlug = gepprofiles.MustRegistrySlug("default")
	}
	if strings.TrimSpace(o.CurrentProfileCookieName) == "" {
		o.CurrentProfileCookieName = "chat_profile"
	}
}
