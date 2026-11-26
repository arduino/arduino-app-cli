// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotifyReady(t *testing.T) {
	t.Run("sends READY=1 to the socket", func(t *testing.T) {
		socket := filepath.Join(t.TempDir(), "notify.sock")
		conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socket, Net: "unixgram"})
		require.NoError(t, err)
		defer conn.Close()
		t.Setenv("NOTIFY_SOCKET", socket)

		require.NoError(t, notifyReady())

		buf := make([]byte, 64)
		n, err := conn.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "READY=1", string(buf[:n]))
		_, set := os.LookupEnv("NOTIFY_SOCKET")
		require.False(t, set, "the processes the daemon runs must not inherit the socket")
	})

	t.Run("does nothing without systemd", func(t *testing.T) {
		t.Setenv("NOTIFY_SOCKET", "")

		require.NoError(t, notifyReady())
	})
}
