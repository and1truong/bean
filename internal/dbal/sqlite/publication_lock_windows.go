//go:build windows

package sqlite

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func (d *DB) WithPublicationLock(ctx context.Context, _ string, operation func() error) error {
	file, err := os.OpenFile(d.publicationLockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	overlapped := new(windows.Overlapped)
	for {
		err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
	return operation()
}
