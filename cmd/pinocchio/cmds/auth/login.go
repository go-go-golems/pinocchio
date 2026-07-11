package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	geppettoauth "github.com/go-go-golems/geppetto/pkg/steps/ai/credentials/oauth"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	glazedsettings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

const callbackPath = "/oauth/callback"

type LoginCommand struct {
	*cmds.CommandDescription
	deps loginDependencies
}

type LoginSettings struct {
	TimeoutSeconds int  `glazed:"timeout-seconds"`
	OpenBrowser    bool `glazed:"open-browser"`
}

type loginDependencies struct {
	listen      func(network, address string) (net.Listener, error)
	openBrowser func(string) error
}

func defaultLoginDependencies() loginDependencies {
	return loginDependencies{
		listen:      net.Listen,
		openBrowser: openBrowser,
	}
}

var _ cmds.GlazeCommand = (*LoginCommand)(nil)

// NewLoginCommand creates the Glazed auth-login verb. Its profile and config
// fields come from Pinocchio's shared profile/bootstrap sections, while the
// command emits only non-secret structured success metadata.
func NewLoginCommand() (*LoginCommand, error) {
	glazedSection, err := glazedsettings.NewGlazedSection()
	if err != nil {
		return nil, err
	}
	commandSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		return nil, err
	}
	return &LoginCommand{
		CommandDescription: cmds.NewCommandDescription(
			"login",
			cmds.WithShort("Log in to an OAuth-backed profile through a local browser callback"),
			cmds.WithLong(`Log in to the selected OAuth-backed profile using Authorization Code with PKCE.

The profile must come from exactly one direct YAML profile registry with mode
0600. The command binds an exact 127.0.0.1 callback before opening a browser,
checks state once, exchanges the authorization code, and saves the resulting
credential tuple without printing it.`),
			cmds.WithFlags(
				fields.New("timeout-seconds", fields.TypeInteger,
					fields.WithDefault(180),
					fields.WithHelp("Maximum time to wait for the browser callback"),
				),
				fields.New("open-browser", fields.TypeBool,
					fields.WithDefault(true),
					fields.WithHelp("Open the authorization URL in the system browser; must remain enabled"),
				),
			),
			cmds.WithSections(glazedSection, commandSettingsSection, profileSettingsSection),
		),
		deps: defaultLoginDependencies(),
	}, nil
}

func (c *LoginCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *values.Values, gp middlewares.Processor) error {
	settings := &LoginSettings{TimeoutSeconds: 180, OpenBrowser: true}
	if parsed != nil {
		if err := parsed.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
			return fmt.Errorf("decode auth login settings: %w", err)
		}
	}
	if settings.TimeoutSeconds <= 0 {
		return errors.New("OAuth login timeout-seconds must be positive")
	}
	if !settings.OpenBrowser {
		return errors.New("OAuth login requires open-browser; manual authorization URL output is disabled")
	}

	commandSettings := &cli.CommandSettings{}
	profileSettings := &profilebootstrap.ProfileSettings{}
	if parsed != nil {
		if err := parsed.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err != nil {
			return fmt.Errorf("decode auth login command settings: %w", err)
		}
		if err := parsed.DecodeSectionInto(profilebootstrap.ProfileSettingsSectionSlug, profileSettings); err != nil {
			return fmt.Errorf("decode auth login profile settings: %w", err)
		}
	}
	selection, err := profilebootstrap.NewCLISelectionValues(profilebootstrap.CLISelectionInput{
		ConfigFile:        commandSettings.ConfigFile,
		Profile:           profileSettings.Profile,
		ProfileRegistries: profileSettings.ProfileRegistries,
	})
	if err != nil {
		return err
	}
	resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, selection)
	if err != nil {
		return err
	}
	if resolved.Close != nil {
		defer resolved.Close()
	}
	oauthProfile, err := profilebootstrap.ResolveOAuthProfile(ctx, resolved)
	if err != nil {
		return err
	}
	if oauthProfile == nil {
		return errors.New("selected profile is not an OAuth profile")
	}
	if err := runLogin(ctx, oauthProfile, time.Duration(settings.TimeoutSeconds)*time.Second, c.deps); err != nil {
		return err
	}
	return gp.AddRow(ctx, types.NewRow(
		types.MRP("profile", resolved.ResolvedEngineProfile.EngineProfileSlug.String()),
		types.MRP("registry", resolved.ResolvedEngineProfile.RegistrySlug.String()),
		types.MRP("status", "completed"),
	))
}

func runLogin(parent context.Context, profile *profilebootstrap.ResolvedOAuthProfile, timeout time.Duration, deps loginDependencies) error {
	if profile == nil {
		return errors.New("resolved OAuth profile is required")
	}
	if timeout <= 0 {
		return errors.New("OAuth login timeout must be positive")
	}
	if deps.listen == nil || deps.openBrowser == nil {
		return errors.New("OAuth login dependencies are incomplete")
	}
	listener, err := deps.listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("bind OAuth loopback callback: %w", err)
	}
	defer func() { _ = listener.Close() }()

	callbackURL := "http://" + listener.Addr().String() + callbackPath
	client, err := profile.NewOAuthClient(callbackURL)
	if err != nil {
		return err
	}
	state, err := randomState()
	if err != nil {
		return err
	}
	pkce := geppettoauth.NewPKCE()
	authorizationURL, err := client.AuthorizationURL(state, pkce)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	result := make(chan callbackResult, 1)
	server := &http.Server{
		Handler:           callbackHandler(state, result),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		select {
		case <-serveDone:
		case <-time.After(time.Second):
		}
	}()

	if err := deps.openBrowser(authorizationURL); err != nil {
		return errors.New("open OAuth authorization URL in browser")
	}

	var callback callbackResult
	select {
	case <-ctx.Done():
		return errors.New("OAuth browser callback timed out or was cancelled")
	case callback = <-result:
		if callback.err != nil {
			return callback.err
		}
	}
	credential, err := client.ExchangeAuthorizationCode(ctx, callback.code, pkce)
	if err != nil {
		return err
	}
	if err := profile.Store.Save(ctx, profile.Request, credential); err != nil {
		return errors.New("save OAuth credential")
	}
	return nil
}

type callbackResult struct {
	code string
	err  error
}

func callbackHandler(expectedState string, result chan<- callbackResult) http.Handler {
	var once sync.Once
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != callbackPath {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		callback := callbackResult{}
		if r.URL.Query().Get("error") != "" {
			callback.err = errors.New("OAuth provider returned an authorization error")
		} else if !statesEqual(expectedState, r.URL.Query().Get("state")) {
			callback.err = errors.New("OAuth callback state did not match")
		} else if strings.TrimSpace(r.URL.Query().Get("code")) == "" {
			callback.err = errors.New("OAuth callback did not include an authorization code")
		} else {
			callback.code = r.URL.Query().Get("code")
		}
		delivered := false
		once.Do(func() {
			result <- callback
			delivered = true
		})
		if !delivered {
			http.Error(w, "OAuth callback already received", http.StatusConflict)
			return
		}
		if callback.err != nil {
			http.Error(w, "OAuth callback rejected", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "Authorization received. You can return to Pinocchio.")
	})
}

func randomState() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", errors.New("generate OAuth authorization state")
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func statesEqual(expected, actual string) bool {
	if expected == "" || actual == "" || len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func openBrowser(rawURL string) error {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return err
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", rawURL)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		command = exec.Command("xdg-open", rawURL)
	}
	return command.Start()
}
