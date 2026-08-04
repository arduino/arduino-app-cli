// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package board_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/testtools"
	"github.com/arduino/arduino-app-cli/pkg/board"
	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
)

func TestExecAsRoot(t *testing.T) {
	name, adbPort, sshPort := testtools.StartAdbDContainer(t)
	t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })

	conns := []struct {
		name     string
		conn     remote.RemoteConn
		password string
	}{
		{
			name: "adb",
			conn: func() remote.RemoteConn {
				conn, err := adb.FromHost("localhost:"+adbPort, "")
				require.NoError(t, err)
				return conn
			}(),
			password: "arduino",
		},
		{
			name: "ssh",
			conn: func() remote.RemoteConn {
				conn, err := ssh.FromHost("arduino", "arduino", "127.0.0.1:"+sshPort)
				require.NoError(t, err)
				return conn
			}(),
			password: "arduino",
		},
	}

	for _, tc := range conns {
		t.Run(tc.name, func(t *testing.T) {
			out, err := board.ExecAsRoot(tc.conn, tc.password, "whoami")
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(string(out), "root"))
		})
	}
}
