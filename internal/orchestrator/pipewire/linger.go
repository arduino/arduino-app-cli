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
	"strconv"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
)

func setOwner(ownerPath string, systemdMtime time.Time) error {
	if err := os.MkdirAll(filepath.Dir(ownerPath), 0700); err != nil {
		return err
	}
	return os.WriteFile(ownerPath, []byte(systemdMtime.Format(time.RFC3339Nano)), 0600)
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

func lookupUsername(uid int) (string, error) {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

func systemdLingerMtime(username string) (time.Time, error) {
	info, err := os.Stat(filepath.Join("/var/lib/systemd/linger", username))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func EnableLinger(ctx context.Context, cfg config.Configuration) error {
    user, err := user.Current()
    
	alreadyOn, err := isLingerEnabled(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("check linger: %w", err)
	}
	if alreadyOn {
		return nil
	}

	if err := setLinger(ctx, uid, "enable-linger"); err != nil {
		return fmt.Errorf("enable linger: %w", err)
	}
	username, err := lookupUsername(uid)
	if err != nil {
		return fmt.Errorf("look up username: %w", err)
	}
	mtime, err := systemdLingerMtime(user.Username)
	if err != nil {
		return fmt.Errorf("read linger state: %w", err)
	}

	ownerPath := dataDir.Join("pipewire").Join(fmt.Sprintf("linger-owner-%d", uid)).String()
	return setOwner(ownerPath, mtime)
}

func DisableLinger(ctx context.Context, dataDir paths.Path) error {

	ownerPath := dataDir.Join("pipewire").Join(fmt.Sprintf("linger-owner-%d", uid)).String()
	owned, recordedMtime, err := isSetbyApp(ownerPath)
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
		if err := setLinger(ctx, uid, "disable-linger"); err != nil {
			return fmt.Errorf("disable linger: %w", err)
		}
		slog.Debug("linger disabled", slog.String("username", username), slog.Int("uid", uid))
	}

	return clearOwner(ownerPath)
}

// isLingerEnabled reports whether logind linger is currently on for uid,
// using `loginctl show-user ... -p Linger`
func isLingerEnabled(ctx context.Context, uid int) (bool, error) {
	out, err := runLoginctl(ctx, "show-user", strconv.Itoa(uid), "-p", "Linger", "--value")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such user") {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "yes", nil
}

// setLinger enables or disables logind linger for uid via `loginctl
// enable-linger`/`loginctl disable-linger`
func setLinger(ctx context.Context, uid int, cmd string) error {
	_, err := runLoginctl(ctx, cmd, strconv.Itoa(uid))
	return err
}

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
