package main

import (
	"context"
	"embed"
	"io"
	"io/fs"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/webchatcmd"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
)

//go:embed static
var staticFS embed.FS

type Command struct {
	*cmds.CommandDescription
	staticFS fs.FS
}

const webChatCLIAppName = "pinocchio"

func NewCommand(staticFS fs.FS) (*Command, error) {
	profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		return nil, errors.Wrap(err, "create web-chat profile settings section")
	}
	clientSection, err := settings.NewClientValueSection()
	if err != nil {
		return nil, errors.Wrap(err, "create web-chat ai-client section")
	}
	redisLayer, err := rediscfg.NewParameterLayer()
	if err != nil {
		return nil, err
	}
	desc := cmds.NewCommandDescription(
		"web-chat",
		cmds.WithShort("Serve a minimal WebSocket web UI that streams chat events"),
		cmds.WithFlags(
			fields.New("addr", fields.TypeString, fields.WithDefault(":8080"), fields.WithHelp("HTTP listen address")),
			fields.New("idle-timeout-seconds", fields.TypeInteger, fields.WithDefault(60), fields.WithHelp("Stop per-conversation reader after N seconds with no sockets (0=disabled)")),
			fields.New("evict-idle-seconds", fields.TypeInteger, fields.WithDefault(300), fields.WithHelp("Evict conversations after N seconds idle (0=disabled)")),
			fields.New("evict-interval-seconds", fields.TypeInteger, fields.WithDefault(60), fields.WithHelp("Sweep idle conversations every N seconds (0=disabled)")),
			fields.New("root", fields.TypeString, fields.WithDefault("/"), fields.WithHelp("Serve the chat UI under a given URL root (e.g., /chat)")),
			fields.New("timeline-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable timeline snapshots (enables GET /timeline); preferred over timeline-db")),
			fields.New("timeline-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable timeline snapshots (enables GET /timeline); DSN is derived with WAL/busy_timeout")),
			fields.New("turns-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable turn snapshots (enables GET /turns); preferred over turns-db")),
			fields.New("turns-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable turn snapshots (enables GET /turns); DSN is derived with WAL/busy_timeout")),
		),
		cmds.WithSections(profileSettingsSection, clientSection, redisLayer),
	)
	return &Command{CommandDescription: desc, staticFS: staticFS}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *values.Values, _ io.Writer) error {
	return webchatcmd.Run(ctx, parsed, c.staticFS)
}

func main() {
	root := &cobra.Command{Use: "web-chat", PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromCobra(cmd)
	}}

	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, root)

	if err := logging.AddLoggingSectionToRootCommand(root, "pinocchio"); err != nil {
		cobra.CheckErr(err)
	}

	c, err := NewCommand(staticFS)
	cobra.CheckErr(err)
	command, err := cli.BuildCobraCommand(c, cli.WithParserConfig(cli.CobraParserConfig{
		// Hidden base-settings parsing owns config-file loading so we can
		// reuse pinocchio config conventions without exposing AI flags.
		AppName: webChatCLIAppName,
	}))
	cobra.CheckErr(err)
	for _, name := range []string{"print-yaml", "print-parsed-fields", "print-schema"} {
		if flag := command.Flags().Lookup(name); flag != nil {
			flag.Hidden = true
		}
	}
	root.AddCommand(command)
	cobra.CheckErr(root.Execute())
}
