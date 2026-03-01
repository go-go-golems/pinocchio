package main

import (
	"path/filepath"
	"strings"

	"github.com/dop251/goja_nodejs/require"
	"github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func normalizeTimelineJSScriptPaths(raw []string) []string {
	paths := make([]string, 0, len(raw))
	for _, entry := range raw {
		for _, token := range strings.Split(entry, ",") {
			p := strings.TrimSpace(token)
			if p == "" {
				continue
			}
			paths = append(paths, p)
		}
	}
	return paths
}

func configureTimelineJSScripts(rawPaths []string) error {
	paths := normalizeTimelineJSScriptPaths(rawPaths)
	webchat.ClearTimelineRuntime()
	if len(paths) == 0 {
		return nil
	}

	opts := webchat.JSTimelineRuntimeOptions{}
	globalFolders := timelineJSGlobalFolders(paths)
	if len(globalFolders) > 0 {
		opts.RequireOptions = append(opts.RequireOptions, require.WithGlobalFolders(globalFolders...))
	}
	runtime, err := webchat.NewJSTimelineRuntimeWithOptions(opts)
	if err != nil {
		return errors.Wrap(err, "initialize timeline JS runtime")
	}
	for _, path := range paths {
		if err := runtime.LoadScriptFile(path); err != nil {
			return errors.Wrapf(err, "load timeline JS script %q", path)
		}
	}

	webchat.SetTimelineRuntime(runtime)
	log.Info().Strs("scripts", paths).Msg("loaded JS timeline runtime scripts")
	return nil
}

func timelineJSGlobalFolders(paths []string) []string {
	folders := make([]string, 0, len(paths)*2)
	seen := map[string]struct{}{}
	for _, p := range paths {
		dir := strings.TrimSpace(filepath.Dir(strings.TrimSpace(p)))
		if dir == "" || dir == "." {
			continue
		}
		if _, ok := seen[dir]; !ok {
			seen[dir] = struct{}{}
			folders = append(folders, dir)
		}
		nodeModules := filepath.Join(dir, "node_modules")
		if _, ok := seen[nodeModules]; !ok {
			seen[nodeModules] = struct{}{}
			folders = append(folders, nodeModules)
		}
	}
	return folders
}
