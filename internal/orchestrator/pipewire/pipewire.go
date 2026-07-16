// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package pipewire brings up the PipeWire audio stack for the current
// process's own user. user@<uid>.service's own lifecycle is left entirely to
// logind, driven by linger (see EnableLinger/DisableLinger in linger.go) —
// this package only waits for the resulting user bus to appear and then
// enables/starts the PipeWire units themselves (pipewire.socket,
// pipewire-pulse.socket, wireplumber.service), which logind has no notion of.
package pipewire

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	userBusDialAttempts = 30
	userBusDialInterval = 500 * time.Millisecond
)

var pipewireUnits = []string{"pipewire.socket", "pipewire-pulse.socket", "wireplumber.service"}

// StartPipewire waits for the current user's bus to come up — brought up by
// EnableLinger, which callers are expected to have called first — and
// enables/starts the PipeWire units on it.
func StartPipewire(ctx context.Context) error {
	uid := os.Getuid()

	conn, err := waitForUserBus(ctx, uid, userBusDialAttempts, userBusDialInterval)
	if err != nil {
		return fmt.Errorf("dial user bus: %w", err)
	}
	defer conn.Close()

	systemd := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")

	if call := systemd.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.Reload", 0); call.Err != nil {
		return fmt.Errorf("reload: %w", call.Err)
	}

	if call := systemd.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.EnableUnitFiles", 0,
		pipewireUnits, false, true); call.Err != nil {
		return fmt.Errorf("enable unit files: %w", call.Err)
	}

	for _, unit := range pipewireUnits {
		if active, _ := unitActive(ctx, conn, unit); active {
			continue
		}
		if call := systemd.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.StartUnit", 0,
			unit, "replace"); call.Err != nil {
			return fmt.Errorf("start %s: %w", unit, call.Err)
		}
	}

	return nil
}

func unitActive(ctx context.Context, conn *dbus.Conn, name string) (bool, error) {
	systemd := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")

	var unitPath dbus.ObjectPath
	call := systemd.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.GetUnit", 0, name)
	if call.Err != nil {
		return false, call.Err // unit not loaded — treat as not-running by caller
	}
	if err := call.Store(&unitPath); err != nil {
		return false, err
	}

	unitObj := conn.Object("org.freedesktop.systemd1", unitPath)
	v, err := unitObj.GetProperty("org.freedesktop.systemd1.Unit.ActiveState")
	if err != nil {
		return false, err
	}
	state, _ := v.Value().(string)
	return state == "active" || state == "listening" || state == "reloading", nil
}

func dialUserBus(uid int) (*dbus.Conn, error) {
	addr := fmt.Sprintf("unix:path=/run/user/%d/bus", uid)
	conn, err := dbus.Dial(addr)
	if err != nil {
		return nil, err
	}
	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func waitForUserBus(ctx context.Context, uid int, attempts int, delay time.Duration) (*dbus.Conn, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		conn, err := dialUserBus(uid)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}
