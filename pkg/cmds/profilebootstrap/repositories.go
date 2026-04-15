package profilebootstrap

import (
	"context"

	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/pkg/errors"
)

const RepositorySettingsSectionSlug = "pinocchio-app"

type RepositorySettings struct {
	Repositories []string `glazed:"repositories"`
}

func NewRepositorySettingsSection() (schema.Section, error) {
	return schema.NewSection(
		RepositorySettingsSectionSlug,
		"Pinocchio app settings",
		schema.WithFields(
			fields.New("repositories", fields.TypeStringList, fields.WithHelp("Additional prompt repository directories")),
		),
	)
}

func MapPinocchioRepositoriesConfig(rawConfig interface{}) (map[string]map[string]interface{}, error) {
	configMap, ok := rawConfig.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("expected map[string]interface{}, got %T", rawConfig)
	}

	repositories := []string{}
	if rawRepos, ok := configMap["repositories"].([]interface{}); ok {
		for _, repo := range rawRepos {
			if repoStr, ok := repo.(string); ok && repoStr != "" {
				repositories = append(repositories, repoStr)
			}
		}
	}

	return map[string]map[string]interface{}{
		RepositorySettingsSectionSlug: {
			"repositories": repositories,
		},
	}, nil
}

func ResolveRepositoryPaths() ([]string, error) {
	plan := glazedconfig.NewPlan(
		glazedconfig.WithLayerOrder(glazedconfig.LayerSystem, glazedconfig.LayerUser),
		glazedconfig.WithDedupePaths(),
	).Add(
		glazedconfig.SystemAppConfig("pinocchio").Named("system-app-config").Kind("app-config"),
		glazedconfig.HomeAppConfig("pinocchio").Named("home-app-config").Kind("app-config"),
		glazedconfig.XDGAppConfig("pinocchio").Named("xdg-app-config").Kind("app-config"),
	)
	files, _, err := plan.Resolve(context.Background())
	if err != nil {
		return nil, err
	}

	section, err := NewRepositorySettingsSection()
	if err != nil {
		return nil, err
	}
	parsed := values.New()
	if err := sources.Execute(
		schema.NewSchema(schema.WithSections(section)),
		parsed,
		sources.FromResolvedFiles(
			files,
			sources.WithConfigFileMapper(MapPinocchioRepositoriesConfig),
			sources.WithParseOptions(fields.WithSource("config")),
		),
	); err != nil {
		return nil, err
	}

	settings := &RepositorySettings{}
	if err := parsed.DecodeSectionInto(RepositorySettingsSectionSlug, settings); err != nil {
		return nil, err
	}
	return append([]string(nil), settings.Repositories...), nil
}
