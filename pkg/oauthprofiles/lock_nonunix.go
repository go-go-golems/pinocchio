//go:build !unix && !windows

package oauthprofiles

import "errors"

func withRegistryLock(_ string, _ bool, _ func() error) error {
	return errors.New("OAuth profile YAML persistence requires an operating-system file lock")
}

func syncDirectory(_ string) error {
	return nil
}
