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

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// enableLingering enables lingering for the current user if not already enabled, and saves the mtime of the linger file to the configuration directory.
func enableLingering(ctx context.Context, cfg config.Configuration) error {
	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}

	alreadyOn, err := runShowLingerCommand(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("check linger: %w", err)
	}
	if alreadyOn {
		return nil
	}

	err = runEnableLingerCommand(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("enable linger: %w", err)
	}
	mtime, err := getLingerFileMtime(user.Username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}

	return saveLingerRecord(cfg, mtime)
}

func stopLingering(ctx context.Context, cfg config.Configuration) error {
	owned, recordedMtime, err := readLingerRecord(cfg)
	if err != nil {
		return fmt.Errorf("check linger ownership: %w", err)
	}
	if !owned {
		return nil
	}

	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}

	currentMtime, err := getLingerFileMtime(user.Username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}

	if currentMtime.Equal(recordedMtime) {
		err := runDisableLingerCommand(ctx, user.Username)
		if err != nil {
			return fmt.Errorf("disable linger: %w", err)
		}
		slog.Debug("linger disabled", slog.String("username", user.Username))
	}

	return clearLingerRecord(cfg)
}

func lingerRecordPath(cfg config.Configuration) *paths.Path {
	return cfg.DataDir().Join("pipewire_linger.timestamp")
}

func saveLingerRecord(cfg config.Configuration, lingerMtime time.Time) error {
	p := lingerRecordPath(cfg)
	if err := p.Parent().MkdirAll(); err != nil {
		return err
	}
	return fatomic.WriteFile(p.String(), []byte(lingerMtime.Format(time.RFC3339Nano)), 0600)
}

func readLingerRecord(cfg config.Configuration) (bool, time.Time, error) {
	data, err := lingerRecordPath(cfg).ReadFile()
	if err != nil {
		if os.IsNotExist(err) {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, err
	}

	if t, err := time.Parse(time.RFC3339Nano, string(data)); err != nil {
		return false, time.Time{}, fmt.Errorf("parse recorded linger mtime: %w", err)
	} else {
		return true, t, nil
	}
}

func clearLingerRecord(cfg config.Configuration) error {
	err := lingerRecordPath(cfg).Remove()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// getLingerFileMtime returns the mtime of the linger file for the given username, or zero time if the file does not exist.
func getLingerFileMtime(username string) (time.Time, error) {
	info, err := os.Stat(filepath.Join("/var/lib/systemd/linger", username))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func runEnableLingerCommand(ctx context.Context, username string) error {
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

func runDisableLingerCommand(ctx context.Context, username string) error {
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

func runShowLingerCommand(ctx context.Context, username string) (bool, error) {
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
