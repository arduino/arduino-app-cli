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
	"time"

	"github.com/godbus/dbus/v5"
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

func isLingerEnabled(ctx context.Context, uid int) (bool, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return false, err
	}
	defer conn.Close()

	logind := conn.Object("org.freedesktop.login1", dbus.ObjectPath("/org/freedesktop/login1"))
	var userPath dbus.ObjectPath
	call := logind.CallWithContext(ctx, "org.freedesktop.login1.Manager.GetUser", 0, uint32(uid))
	if call.Err != nil {
		if isNoSuchUser(call.Err) {
			return false, nil
		}
		return false, call.Err
	}
	if err := call.Store(&userPath); err != nil {
		return false, err
	}

	userObj := conn.Object("org.freedesktop.login1", userPath)
	v, err := userObj.GetProperty("org.freedesktop.login1.User.Linger")
	if err != nil {
		return false, err
	}
	enabled, _ := v.Value().(bool)
	return enabled, nil
}

func setLinger(ctx context.Context, uid int, enable bool) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	logind := conn.Object("org.freedesktop.login1", dbus.ObjectPath("/org/freedesktop/login1"))
	call := logind.CallWithContext(ctx, "org.freedesktop.login1.Manager.SetUserLinger", 0, uint32(uid), enable, false)
	return call.Err
}

func isNoSuchUser(err error) bool {
	dbusErr, ok := err.(dbus.Error)
	return ok && dbusErr.Name == "org.freedesktop.login1.NoSuchUser"
}
