package main

import (
	"strings"

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

	runtime := webchat.NewJSTimelineRuntime()
	for _, path := range paths {
		if err := runtime.LoadScriptFile(path); err != nil {
			return errors.Wrapf(err, "load timeline JS script %q", path)
		}
	}

	webchat.SetTimelineRuntime(runtime)
	log.Info().Strs("scripts", paths).Msg("loaded JS timeline runtime scripts")
	return nil
}
