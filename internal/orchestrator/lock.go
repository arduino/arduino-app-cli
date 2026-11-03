// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/arduino/go-paths-helper"
	"golang.org/x/sys/unix"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

const (
	appsLockFileName = "apps.lock"
	lockPollInterval = 50 * time.Millisecond
)

// withAppsLock runs fn holding the lock that serializes the app operations of the CLI and the
// daemon. A process that dies releases it: the kernel holds the lock, the file is only its name.
func withAppsLock(ctx context.Context, cfg config.Configuration, fn func() error) error {
	unlock, err := lockFile(ctx, cfg.DataDir().Join(appsLockFileName))
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// lockFile takes an exclusive flock on path, polling so that ctx can cancel the wait.
func lockFile(ctx context.Context, path *paths.Path) (func(), error) {
	if err := path.Parent().MkdirAll(); err != nil {
		return nil, fmt.Errorf("failed to create the lock folder: %w", err)
	}
	// Read only: taking the lock needs no write access to the file.
	file, err := os.OpenFile(path.String(), os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open the lock file: %w", err)
	}
	fd := int(file.Fd()) //nolint:gosec // a file descriptor fits an int
	waiting := false
	for {
		err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() {
				_ = unix.Flock(fd, unix.LOCK_UN)
				_ = file.Close()
			}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			_ = file.Close()
			return nil, fmt.Errorf("failed to lock %s: %w", path, err)
		}
		if !waiting {
			slog.Info("waiting for another app operation to finish", slog.String("lock", path.String()))
			waiting = true
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(lockPollInterval):
		}
	}
}
