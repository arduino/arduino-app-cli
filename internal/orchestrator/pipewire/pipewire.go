// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package pipewire manages the PipeWire audio stack's systemd --user lifecycle
// for the current process's own user, via direct D-Bus calls to systemd and
// logind
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

const userManagerUnit = "user@%d.service"

func StartPipewire(ctx context.Context) error {
	uid := currentUID()

	active, err := checkActive(ctx, uid)
	if err != nil {
		return fmt.Errorf("check active: %w", err)
	}
	if active {
		return nil
	}

	if err := startUserManager(ctx, uid); err != nil {
		return fmt.Errorf("start user manager: %w", err)
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

// StopPipewire stops the PipeWire ecosystem for the current user by
// stopping user@<uid>.service outright — but only if EnsureRunning was the
// one that started it, and only if no other logind session for that user is
// currently active (stopping the user manager would otherwise pull audio
// out from under a real lightdm/SSH login). Stopping user@<uid>.service
// itself cascades: systemd --user runs its own shutdown transaction first,
// stopping pipewire/wireplumber along with everything else it manages, so
// there's no need to stop those units individually beforehand.
func StopPipewire(ctx context.Context) error {
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
		if err := stopUserManager(ctx, uid); err != nil {
			return fmt.Errorf("stop user manager: %w", err)
		}
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

// startUserManager starts user@<uid>.service on the system bus, the same
// unit logind itself would start on a real login (or previously, on
// enabling linger) — bringing up the user's systemd --user instance
// (runtime dir, D-Bus user bus, etc.) without going through a logind
// session or linger at all.
func startUserManager(ctx context.Context, uid uint32) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	unit := fmt.Sprintf(userManagerUnit, uid)
	systemd := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	call := systemd.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.StartUnit", 0, unit, "replace")
	return call.Err
}

// stopUserManager stops user@<uid>.service on the system bus. systemd --user
// runs its own shutdown transaction first, stopping every unit it manages
// (pipewire, wireplumber, ...) before the manager itself exits.
func stopUserManager(ctx context.Context, uid uint32) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	unit := fmt.Sprintf(userManagerUnit, uid)
	systemd := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	call := systemd.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.StopUnit", 0, unit, "replace")
	if call.Err != nil && !isNoSuchUnit(call.Err) {
		return call.Err
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

// sessionInfo mirrors logind's ListSessions reply struct field-for-field —
// godbus decodes the D-Bus struct array positionally into these fields.
type sessionInfo struct {
	ID       string
	UID      uint32
	UserName string
	Seat     string
	Path     dbus.ObjectPath
}

// nonHumanSessionClasses are logind session classes that exist purely as
// bookkeeping for a systemd --user manager instance, not for an actual
// logged-in person. Starting user@<uid>.service — which EnsureRunning does
// on every run — always creates one of these for its own uid, so without
// filtering them out, hasActiveSession would report "another session is
// active" for the very session we just started ourselves, and
// TeardownIfUnneeded would never be able to stop anything.
var nonHumanSessionClasses = map[string]bool{
	"manager":       true,
	"manager-early": true,
	"background":    true,
}

// hasActiveSession reports whether a real, human logind session (lightdm,
// SSH, a console login, ...) is currently active for uid, as opposed to the
// bookkeeping session that merely reflects user@<uid>.service running.
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
		if s.UID != uid {
			continue
		}
		class, err := sessionClass(conn, s.Path)
		if err != nil {
			return false, fmt.Errorf("get class of session %s: %w", s.ID, err)
		}
		if !nonHumanSessionClasses[class] {
			return true, nil
		}
	}
	return false, nil
}

func sessionClass(conn *dbus.Conn, path dbus.ObjectPath) (string, error) {
	session := conn.Object("org.freedesktop.login1", path)
	v, err := session.GetProperty("org.freedesktop.login1.Session.Class")
	if err != nil {
		return "", err
	}
	class, _ := v.Value().(string)
	return class, nil
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
