// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package pipewire

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
)

const systemdLingerDir = "/var/lib/systemd/linger"

func ownerPath(uid int) string {
	return filepath.Join(os.Getenv("ARDUINO_APP_CLI__DATA_DIR"), "pipewire", fmt.Sprintf("linger-owner-%d", uid))
}

func setOwner(uid int, systemdMtime time.Time) error {
	path := ownerPath(uid)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(systemdMtime.Format(time.RFC3339Nano)), 0600)
}

func isSetbyApp(uid int) (owned bool, recordedMtime time.Time, err error) {
	data, err := os.ReadFile(ownerPath(uid))
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

func clearOwner(uid int) error {
	err := os.Remove(ownerPath(uid))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func lookupUsername(uid int) (string, error) {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

func systemdLingerMtime(username string) (time.Time, error) {
	info, err := os.Stat(filepath.Join(systemdLingerDir, username))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func EnableLinger(ctx context.Context) error {
	uid := os.Getuid()

	alreadyOn, err := isLingerEnabled(ctx, uid)
	if err != nil {
		return fmt.Errorf("check linger: %w", err)
	}
	if alreadyOn {
		return nil
	}

	if err := setLinger(ctx, uid, true); err != nil {
		return fmt.Errorf("enable linger: %w", err)
	}

	username, err := lookupUsername(uid)
	if err != nil {
		return fmt.Errorf("look up username: %w", err)
	}
	mtime, err := systemdLingerMtime(username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}
	return setOwner(uid, mtime)
}

func DisableLinger(ctx context.Context) error {
	uid := os.Getuid()

	owned, recordedMtime, err := isSetbyApp(uid)
	if err != nil {
		return fmt.Errorf("check linger ownership: %w", err)
	}
	if !owned {
		return nil
	}

	username, err := lookupUsername(uid)
	if err != nil {
		return fmt.Errorf("look up username: %w", err)
	}
	currentMtime, err := systemdLingerMtime(username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}

	if currentMtime.Equal(recordedMtime) {
		if err := setLinger(ctx, uid, false); err != nil {
			return fmt.Errorf("disable linger: %w", err)
		}
	}
	return clearOwner(uid)
}

// isLingerEnabled reports whether logind linger is currently on for uid,
// using `loginctl show-user ... -p Linger` rather than reading the
// User.Linger property over D-Bus.
func isLingerEnabled(ctx context.Context, uid int) (bool, error) {
	out, err := runLoginctl(ctx, "show-user", strconv.Itoa(uid), "-p", "Linger", "--value")
	if err != nil {
		if isNoSuchUser(err) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "yes", nil
}

// setLinger enables or disables logind linger for uid via `loginctl
// enable-linger`/`loginctl disable-linger` rather than the SetUserLinger
// D-Bus call. Like SetUserLinger, this needs no elevated privilege when
// applied to one's own uid.
func setLinger(ctx context.Context, uid int, enable bool) error {
	sub := "disable-linger"
	if enable {
		sub = "enable-linger"
	}
	_, err := runLoginctl(ctx, sub, strconv.Itoa(uid))
	return err
}

// runLoginctl runs `loginctl <args...>` and returns its combined output.
func runLoginctl(ctx context.Context, args ...string) (string, error) {
	cmdArgs := append([]string{"loginctl"}, args...)
	cmd, err := paths.NewProcess(nil, cmdArgs...)
	if err != nil {
		return "", fmt.Errorf("create process %q: %w", strings.Join(cmdArgs, " "), err)
	}

	out, err := cmd.RunAndCaptureCombinedOutput(ctx)
	if err != nil {
		return string(out), fmt.Errorf("%s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func isNoSuchUser(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such user")
}
