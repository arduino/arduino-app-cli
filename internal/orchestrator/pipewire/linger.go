// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package pipewire

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func setOwner(ownerPath string, lingerTimestamp string) error {
	if err := os.MkdirAll(filepath.Dir(ownerPath), 0700); err != nil {
		return err
	}
	return os.WriteFile(ownerPath, []byte(lingerTimestamp), 0600)
}

func isSetbyApp(ownerPath string) (owned bool, recordedTimestamp string, err error) {
	data, err := os.ReadFile(ownerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, string(data), nil
}

func clearOwner(ownerPath string) error {
	err := os.Remove(ownerPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func systemdLingerTimestamp(ctx context.Context, username string) (string, error) {
	cmdArgs := []string{"loginctl", "show-user", username, "-p", "TimestampMonotonic", "--value"}
	cmd, err := paths.NewProcess(nil, cmdArgs...)
	if err != nil {
		return "", fmt.Errorf("create process %q: %w", strings.Join(cmdArgs, " "), err)
	}

	out, err := cmd.RunAndCaptureCombinedOutput(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func runEnableLingerCmd(ctx context.Context, cfg config.Configuration) error {
	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}

	alreadyOn, err := showLinger(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("check linger: %w", err)
	}
	if alreadyOn {
		return nil
	}

	if err := enableLinger(ctx, user.Username); err != nil {
		return fmt.Errorf("enable linger: %w", err)
	}
	timestamp, err := systemdLingerTimestamp(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}

	ownerPath := cfg.DataDir().Join("pipewire").Join(fmt.Sprintf("linger-owner-%s", user.Username)).String()
	return setOwner(ownerPath, timestamp)
}

func StopIfNotNeeded(ctx context.Context, cfg config.Configuration) error {

	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}

	ownerPath := cfg.DataDir().Join("pipewire").Join(fmt.Sprintf("linger-owner-%s", user.Username)).String()
	owned, recordedTimestamp, err := isSetbyApp(ownerPath)
	if err != nil {
		return fmt.Errorf("check linger ownership: %w", err)
	}
	if !owned {
		return nil
	}

	currentTimestamp, err := systemdLingerTimestamp(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}
	if currentTimestamp == recordedTimestamp {
		if err := disableLinger(ctx, user.Username); err != nil {
			return fmt.Errorf("disable linger: %w", err)
		}
		slog.Debug("linger disabled", slog.String("username", user.Username))
	}

	return clearOwner(ownerPath)
}

func enableLinger(ctx context.Context, username string) error {
	cmdArgs := []string{"loginctl", "enable-linger", username}
	cmd, err := paths.NewProcess(nil, cmdArgs...)
	if err != nil {
		return fmt.Errorf("create process %q: %w", strings.Join(cmdArgs, " "), err)
	}

	if out, err := cmd.RunAndCaptureCombinedOutput(ctx); err != nil {
		return fmt.Errorf("%s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func disableLinger(ctx context.Context, username string) error {
	cmdArgs := []string{"loginctl", "disable-linger", username}
	cmd, err := paths.NewProcess(nil, cmdArgs...)
	if err != nil {
		return fmt.Errorf("create process %q: %w", strings.Join(cmdArgs, " "), err)
	}

	if out, err := cmd.RunAndCaptureCombinedOutput(ctx); err != nil {
		return fmt.Errorf("%s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func showLinger(ctx context.Context, username string) (bool, error) {
	cmdArgs := []string{"loginctl", "show-user", username, "-p", "Linger", "--value"}
	cmd, err := paths.NewProcess(nil, cmdArgs...)
	if err != nil {
		return false, fmt.Errorf("create process %q: %w", strings.Join(cmdArgs, " "), err)
	}

	out, err := cmd.RunAndCaptureCombinedOutput(ctx)
	if err != nil {
		if strings.Contains(string(out), "is not logged in or lingering") {
			return false, nil
		}
		return false, fmt.Errorf("%s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)) == "yes", nil
}
