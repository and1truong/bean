//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package sqlite

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func (d *DB) WithPublicationLock(ctx context.Context, _ string, operation func() error) error {
	file, err := os.OpenFile(d.publicationLockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return operation()
}
