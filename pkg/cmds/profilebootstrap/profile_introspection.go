package profilebootstrap

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

type ProfileIntrospectionSettings = bootstrap.ProfileIntrospectionSettings
type ProfileRegistryReportOptions = bootstrap.ProfileRegistryReportOptions
type ProfileRegistryReport = bootstrap.ProfileRegistryReport

func ResolveProfileIntrospectionSettings(parsed *values.Values) ProfileIntrospectionSettings {
	return bootstrap.ResolveProfileIntrospectionSettings(parsed)
}

func RenderProfileRegistryReport(w interface{ Write([]byte) (int, error) }, report *ProfileRegistryReport, format string) error {
	return bootstrap.RenderProfileRegistryReport(w, report, format)
}

func BuildProfileRegistryReport(ctx context.Context, parsed *values.Values, opts ProfileRegistryReportOptions) (*ProfileRegistryReport, func(), error) {
	runtime, err := ResolveCLIProfileRuntime(ctx, parsed)
	if err != nil {
		return nil, nil, err
	}
	cleanup := runtime.Close
	chain := runtime.ProfileRegistryChain
	if chain == nil || chain.Registry == nil {
		return &bootstrap.ProfileRegistryReport{}, cleanup, nil
	}

	defaultProfileSlug := gepprofiles.EngineProfileSlug("")
	if !chain.DefaultRegistrySlug.IsZero() && chain.DefaultProfileResolve.EngineProfileSlug.IsZero() {
		reg, regErr := chain.Registry.GetRegistry(ctx, chain.DefaultRegistrySlug)
		if regErr == nil && reg != nil && !reg.DefaultEngineProfileSlug.IsZero() {
			defaultProfileSlug = reg.DefaultEngineProfileSlug
		}
	}

	report, err := bootstrap.BuildProfileRegistryReportFromRegistry(ctx, bootstrap.ProfileRegistryReportInput{
		SourceEntries:       runtime.ProfileSettings.ProfileRegistries,
		Registry:            chain.Registry,
		DefaultRegistrySlug: chain.DefaultRegistrySlug,
		DefaultProfileSlug:  defaultProfileSlug,
		ResolveInput:        chain.DefaultProfileResolve,
	}, opts)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, err
	}
	return report, cleanup, nil
}
