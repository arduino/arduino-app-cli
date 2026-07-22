// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package pipewire brings up the PipeWire audio stack for the current
// process's own user. user@<uid>.service's own lifecycle is left entirely to
// logind, driven by linger (see EnableLinger/DisableLinger in linger.go) —
// this package only waits for the resulting user manager to come up and then
// enables/starts the PipeWire units themselves (pipewire.socket,
// pipewire-pulse.socket, wireplumber.service), which logind has no notion of.

package pipewire

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
)

const (
	userManagerDialAttempts = 30
	userManagerDialInterval = 500 * time.Millisecond
)

var pipewireUnits = []string{"pipewire.socket", "pipewire-pulse.socket", "wireplumber.service"}

// StartPipewire waits for the current user's systemd --user manager to come
// up — brought up by EnableLinger, which callers are expected to have called
// first — and enables/starts the PipeWire units on it.
func StartPipewire(ctx context.Context) error {
	if err := waitForUserManager(ctx, userManagerDialAttempts, userManagerDialInterval); err != nil {
		return fmt.Errorf("wait for user systemd manager: %w", err)
	}

	if _, err := runSystemctlUser(ctx, "daemon-reload"); err != nil {
		return fmt.Errorf("reload: %w", err)
	}

	enableUnitsArgs := append([]string{"enable", "--now"}, pipewireUnits...)
	if _, err := runSystemctlUser(ctx, enableUnitsArgs...); err != nil {
		return fmt.Errorf("enable/start unit files: %w", err)
	}

	return nil
}

func waitForUserManager(ctx context.Context, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if _, err := runSystemctlUser(ctx, "daemon-reload"); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

func runSystemctlUser(ctx context.Context, args ...string) (string, error) {
	cmdArgs := append([]string{"systemctl", "--user"}, args...)
	cmdEnv := append(os.Environ(), fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", os.Getuid()))
	cmd, err := paths.NewProcess(cmdEnv, cmdArgs...)
	if err != nil {
		return "", fmt.Errorf("create process %q: %w", strings.Join(cmdArgs, " "), err)
	}

	out, err := cmd.RunAndCaptureCombinedOutput(ctx)
	if err != nil {
		return string(out), fmt.Errorf("%s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
