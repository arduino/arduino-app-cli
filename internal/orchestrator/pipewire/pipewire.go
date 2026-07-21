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
//
// Everything here shells out to systemctl --user rather than talking to
// D-Bus directly, so it behaves exactly like an operator typing the
// equivalent commands by hand.
package pipewire

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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

	// --now both enables the unit files and starts them; systemctl start
	// is already a no-op on a unit that's already active (e.g. one that
	// came up via socket activation), so there's no need for a separate
	// is-active check before starting.
	if _, err := runSystemctlUser(ctx, append([]string{"enable", "--now"}, pipewireUnits...)...); err != nil {
		return fmt.Errorf("enable/start unit files: %w", err)
	}

	return nil
}

// waitForUserManager blocks until the current user's systemd --user manager
// is reachable via systemctl, retrying up to attempts times with delay
// between attempts.
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

// runSystemctlUser runs `systemctl --user <args...>` for the current
// process's own user and returns its combined output. XDG_RUNTIME_DIR is set
// explicitly so the command can find /run/user/<uid>/bus even when the
// daemon isn't running inside a full login session (e.g. over adb shell).
func runSystemctlUser(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)
	cmd.Env = userCommandEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w: %s", cmd.String(), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// userCommandEnv returns the environment systemctl/loginctl need to reach
// the current user's own session.
func userCommandEnv() []string {
	uid := os.Getuid()
	env := os.Environ()
	env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", uid))
	return env
}
