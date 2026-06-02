package webapp

import (
	"io/fs"
	"net/http"

	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/appserver"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/profiles"
)

type MuxOptions struct {
	StaticFS              fs.FS
	AppConfigJS           string
	RequestResolver       *profiles.RequestResolver
	ChatServer            *appserver.Server
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
		mux.HandleFunc("/api/chat/sessions", opts.ChatServer.HandleCreateSession)
		mux.HandleFunc("/api/chat/sessions/", opts.ChatServer.HandleSessionRoutes)
		mux.HandleFunc("/api/chat/ws", opts.ChatServer.HandleWS)
	}
	mux.HandleFunc("/app-config.js", buildAppConfigHandler(opts.AppConfigJS))
	RegisterStaticUIHandlers(mux, opts.StaticFS)
	return mux
}
