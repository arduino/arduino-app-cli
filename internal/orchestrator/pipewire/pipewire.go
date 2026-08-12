// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package pipewire brings up the PipeWire audio stack for the current
// process's own user. user@<uid>.service's own lifecycle is left entirely to
// logind, driven by linger (see EnableLinger/DisableLinger in linger.go)

package pipewire

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

const (
	userManagerDialAttempts = 30
	userManagerDialInterval = 500 * time.Millisecond
)

// EnsurePipewireRunning starts the PipeWire units (pipewire, pipewire-pulse and
// wireplumber) for the current user, enabling lingering first so that logind
// brings up the user's systemd --user manager and keeps it running.
func EnsurePipewireRunning(ctx context.Context, cfg config.Configuration) error {
	if err := enableLingering(ctx, cfg); err != nil {
		return fmt.Errorf("failed to enable audio service linger: %w", err)
	}

	var lastErr error
	for range userManagerDialAttempts {
		if _, err := runStartPipewireUnitsCommand(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(userManagerDialInterval):
		}
	}
	return fmt.Errorf("wait for user systemd manager: %w", lastErr)
}

// StopIfNotNeeded tears down the PipeWire units for the current user, but only
// if EnsurePipewireRunning was the one that started them. It does so by
// disabling lingering, letting logind shut down the user's systemd --user
// manager and the PipeWire units running on it.
func StopIfNotNeeded(ctx context.Context, cfg config.Configuration) error {
	return stopLingering(ctx, cfg)
}

func runStartPipewireUnitsCommand(ctx context.Context) (string, error) {
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", os.Getuid())}
	cmd, err := paths.NewProcess(env,
		"systemctl",
		"--user",
		"start",
		"pipewire.socket",
		"pipewire-pulse.socket",
		"wireplumber.service",
	)
	if err != nil {
		return "", fmt.Errorf("create process start pipewire: %w", err)
	}

	out, err := cmd.RunAndCaptureCombinedOutput(ctx)
	if err != nil {
		return string(out), fmt.Errorf("%s: %w: %s", strings.Join(cmd.GetArgs(), " "), err, strings.TrimSpace(string(out)))
	}

	return string(out), nil
}
