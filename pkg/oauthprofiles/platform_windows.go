//go:build windows

package oauthprofiles

import "errors"

func validateYAMLPersistencePlatform() error {
	return errors.New("OAuth profile YAML persistence is not supported on Windows")
}
