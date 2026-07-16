//go:build unix

package oauthprofiles

import (
	"fmt"
	"os"
	"syscall"
)

func withRegistryLock(path string, exclusive bool, fn func() error) error {
	lock, err := os.OpenFile(path+".oauth.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open OAuth profile lock: %w", err)
	}
	defer func() { _ = lock.Close() }()
	if err := lock.Chmod(0o600); err != nil {
		return fmt.Errorf("set OAuth profile lock mode: %w", err)
	}
	mode := syscall.LOCK_SH
	if exclusive {
		mode = syscall.LOCK_EX
	}
	if err := syscall.Flock(int(lock.Fd()), mode); err != nil {
		return fmt.Errorf("lock OAuth profile registry: %w", err)
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}
