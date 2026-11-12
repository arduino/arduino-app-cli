// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"net"
	"os"
)

// notifyReady tells systemd that the daemon is ready, as a Type=notify service does
// (see sd_notify(3)). It does nothing when the daemon is not run by systemd, and it
// unsets NOTIFY_SOCKET.
func notifyReady() error {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" {
		return nil
	}
	// The daemon notifies once: the processes it runs, as apt or docker, must not inherit the socket.
	_ = os.Unsetenv("NOTIFY_SOCKET")
	// A name starting with "@" is an abstract socket: Go maps it as systemd does.
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: socket, Net: "unixgram"})
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte("READY=1"))
	return err
}
