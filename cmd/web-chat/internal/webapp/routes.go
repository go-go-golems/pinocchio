package webapp

import (
	"io/fs"
	"net/http"

	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/profiles"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/httpapi"
)

type MuxOptions struct {
	StaticFS              fs.FS
	AppConfigJS           string
	RequestResolver       *profiles.RequestResolver
	ChatServer            *httpapi.Server
	MiddlewareDefinitions middlewarecfg.DefinitionRegistry
	ExtensionSchemas      []profiles.ExtensionSchemaDocument
}

func NewMux(opts MuxOptions) *http.ServeMux {
	mux := http.NewServeMux()
	if opts.RequestResolver != nil && opts.RequestResolver.Registry() != nil {
		profiles.RegisterAPIHandlers(mux, opts.RequestResolver.Registry(), profiles.APIOptions{
			DefaultRegistrySlug:             opts.RequestResolver.DefaultRegistrySlug(),
			EnableCurrentProfileCookieRoute: true,
			CurrentProfileCookieName:        "chat_profile",
			MiddlewareDefinitions:           opts.MiddlewareDefinitions,
			ExtensionSchemas:                opts.ExtensionSchemas,
		})
	}
	if opts.ChatServer != nil {
		opts.ChatServer.Register(mux)
	}
	mux.HandleFunc("/app-config.js", buildAppConfigHandler(opts.AppConfigJS))
	RegisterStaticUIHandlers(mux, opts.StaticFS)
	return mux
}
