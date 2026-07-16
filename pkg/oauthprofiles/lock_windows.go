//go:build windows

package oauthprofiles

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
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

	flags := uint32(0)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(lock.Fd()), flags, 0, 1, 0, &overlapped); err != nil {
		return fmt.Errorf("lock OAuth profile registry: %w", err)
	}
	defer func() {
		_ = windows.UnlockFileEx(windows.Handle(lock.Fd()), 0, 1, 0, &overlapped)
	}()
	return fn()
}

func syncDirectory(_ string) error {
	return nil
}
