package bootstrap

import root "github.com/go-go-golems/pinocchio/pkg/webchat"

// Server is the app-composition entry point for webchat.
type Server = root.Server

// RouterOption configures server/router dependencies.
type RouterOption = root.RouterOption

var NewServer = root.NewServer
