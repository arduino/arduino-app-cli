// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package pipewire manages the PipeWire audio stack's systemd --user lifecycle
// for the current process's own user, via direct D-Bus calls to systemd and
// logind (no shelling out to systemctl/loginctl). This only works because the
// arduino-app-cli daemon itself runs as the board's own unprivileged user
// (see debian/arduino-app-cli/etc/systemd/system/arduino-app-cli.service),
// i.e. the exact account whose systemd --user instance runs PipeWire — so
// every D-Bus call here can authenticate as that user directly, no privilege
// dance needed.
package pipewire

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
)

// user@<uid>.service can take several seconds to come up on first start
// (PAM chain, runtime dir mount, D-Bus broker), especially on slower ARM
// boards — give it a generous window rather than the instant a login would.
const (
	userBusDialAttempts = 30
	userBusDialInterval = 500 * time.Millisecond
)

var pipewireUnits = []string{"pipewire.socket", "pipewire-pulse.socket", "wireplumber.service"}

// Stopped in this order (not dependency order — Requires= doesn't propagate
// stops, so each unit is stopped explicitly rather than relying on one stop
// cascading to the others).
var stopUnits = []string{
	"wireplumber.service",
	"pipewire-pulse.service",
	"pipewire-pulse.socket",
	"pipewire.service",
	"pipewire.socket",
}

// EnsureRunning brings up the PipeWire ecosystem for the current user if it
// isn't already active. If it's already active — started by a real
// lightdm/SSH session, or a previous run we never tore down — it's left
// untouched and we don't claim ownership of it.
func EnsureRunning(ctx context.Context) error {
	uid := currentUID()

	active, err := checkActive(ctx, uid)
	if err != nil {
		return fmt.Errorf("check active: %w", err)
	}
	if active {
		return nil
	}

	if err := setLinger(ctx, uid, true); err != nil {
		return fmt.Errorf("enable linger: %w", err)
	}

	conn, err := retryDialUserBus(ctx, uid, userBusDialAttempts, userBusDialInterval)
	if err != nil {
		return fmt.Errorf("dial user bus: %w", err)
	}
	defer conn.Close()

	if err := bootstrap(ctx, conn); err != nil {
		return fmt.Errorf("start units: %w", err)
	}

	return markOwned(uid)
}

// TeardownIfUnneeded stops the PipeWire ecosystem for the current user and
// disables linger — but only if EnsureRunning was the one that started it,
// and only stops the units outright if no other logind session for that
// user is currently active. If another session is present, linger is still
// disabled (so that session's own eventual logout tears things down
// normally) but the units themselves are left running for it.
func TeardownIfUnneeded(ctx context.Context) error {
	uid := currentUID()

	owned, err := isOwned(uid)
	if err != nil {
		return fmt.Errorf("check ownership: %w", err)
	}
	if !owned {
		return nil
	}

	otherSession, err := hasActiveSession(ctx, uid)
	if err != nil {
		return fmt.Errorf("check other sessions: %w", err)
	}

	if !otherSession {
		if err := stopUnitsOnUserBus(ctx, uid); err != nil {
			return fmt.Errorf("stop units: %w", err)
		}
	}

	if err := setLinger(ctx, uid, false); err != nil {
		return fmt.Errorf("disable linger: %w", err)
	}

	return clearOwned(uid)
}

func currentUID() uint32 {
	return uint32(os.Getuid())
}

// checkActive asks whether pipewire.socket is active for uid. A missing bus
// (nobody has started anything yet) is reported as "not active", not an
// error.
func checkActive(ctx context.Context, uid uint32) (bool, error) {
	conn, err := dialUserBus(uid)
	if err != nil {
		return false, nil
	}
	defer conn.Close()
	active, _ := unitActive(ctx, conn, "pipewire.socket")
	return active, nil
}

func stopUnitsOnUserBus(ctx context.Context, uid uint32) error {
	conn, err := retryDialUserBus(ctx, uid, userBusDialAttempts, userBusDialInterval)
	if err != nil {
		return fmt.Errorf("dial user bus: %w", err)
	}
	defer conn.Close()

	systemd := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	for _, unit := range stopUnits {
		call := systemd.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.StopUnit", 0, unit, "replace")
		if call.Err != nil && !isNoSuchUnit(call.Err) {
			return fmt.Errorf("stop %s: %w", unit, call.Err)
		}
	}
	return nil
}

func isNoSuchUnit(err error) bool {
	dbusErr, ok := err.(dbus.Error)
	return ok && dbusErr.Name == "org.freedesktop.systemd1.NoSuchUnit"
}

func bootstrap(ctx context.Context, conn *dbus.Conn) error {
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

func setLinger(ctx context.Context, uid uint32, enable bool) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	logind := conn.Object("org.freedesktop.login1", dbus.ObjectPath("/org/freedesktop/login1"))
	call := logind.CallWithContext(ctx, "org.freedesktop.login1.Manager.SetUserLinger", 0, uid, enable, true)
	return call.Err
}

// sessionInfo mirrors logind's ListSessions reply struct field-for-field —
// godbus decodes the D-Bus struct array positionally into these fields.
type sessionInfo struct {
	ID       string
	UID      uint32
	UserName string
	Seat     string
	Path     dbus.ObjectPath
}

func hasActiveSession(ctx context.Context, uid uint32) (bool, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return false, err
	}
	defer conn.Close()

	logind := conn.Object("org.freedesktop.login1", dbus.ObjectPath("/org/freedesktop/login1"))
	var sessions []sessionInfo
	call := logind.CallWithContext(ctx, "org.freedesktop.login1.Manager.ListSessions", 0)
	if call.Err != nil {
		return false, call.Err
	}
	if err := call.Store(&sessions); err != nil {
		return false, err
	}

	for _, s := range sessions {
		if s.UID == uid {
			return true, nil
		}
	}
	return false, nil
}

func dialUserBus(uid uint32) (*dbus.Conn, error) {
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

func retryDialUserBus(ctx context.Context, uid uint32, attempts int, delay time.Duration) (*dbus.Conn, error) {
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
