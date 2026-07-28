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
	"time"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func setOwner(ownerPath string, lingerMtime time.Time) error {
	if err := os.MkdirAll(filepath.Dir(ownerPath), 0700); err != nil {
		return err
	}
	return os.WriteFile(ownerPath, []byte(lingerMtime.Format(time.RFC3339Nano)), 0600)
}

func isSetbyApp(ownerPath string) (owned bool, recordedMtime time.Time, err error) {
	data, err := os.ReadFile(ownerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, string(data))
	if err != nil {
		return false, time.Time{}, fmt.Errorf("parse recorded linger mtime: %w", err)
	}
	return true, t, nil
}

func clearOwner(ownerPath string) error {
	err := os.Remove(ownerPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func lingerFileMtime(username string) (time.Time, error) {
	info, err := os.Stat(filepath.Join("/var/lib/systemd/linger", username))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return info.ModTime(), nil
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
	mtime, err := lingerFileMtime(user.Username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}

	ownerPath := cfg.DataDir().Join("pipewire").Join(fmt.Sprintf("linger-owner-%s", user.Username)).String()
	return setOwner(ownerPath, mtime)
}

func StopIfNotNeeded(ctx context.Context, cfg config.Configuration) error {

	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}

	ownerPath := cfg.DataDir().Join("pipewire").Join(fmt.Sprintf("linger-owner-%s", user.Username)).String()
	owned, recordedMtime, err := isSetbyApp(ownerPath)
	if err != nil {
		return fmt.Errorf("check linger ownership: %w", err)
	}
	if !owned {
		return nil
	}

	currentMtime, err := lingerFileMtime(user.Username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}

	if currentMtime.Equal(recordedMtime) {
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
