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

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/go-paths-helper"
)

const (
	userManagerDialAttempts = 30
	userManagerDialInterval = 500 * time.Millisecond
)

var pipewireUnits = []string{"pipewire.socket", "pipewire-pulse.socket", "wireplumber.service"}

// EnsurePipewireRunning waits for the current user's systemd --user manager to come
// up — brought up by EnableLinger, which callers are expected to have called
// first — and starts the PipeWire units on it.
func EnsurePipewireRunning(ctx context.Context, cfg config.Configuration) error {

	if err := runEnableLingerCmd(ctx, cfg); err != nil {
		return fmt.Errorf("failed to enable audio service linger: %w", err)
	}

	var lastErr error
	for i := 0; i < userManagerDialAttempts; i++ {
		if _, err := runSystemctlUser(ctx, pipewireUnits...); err == nil {
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

func runSystemctlUser(ctx context.Context, args ...string) (string, error) {
	cmdArgs := append([]string{"systemctl", "--user", "start"}, args...)
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
