// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package board

import (
	"fmt"
	"io"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

func ExecAsRoot(conn remote.RemoteConn, password string, args ...string) ([]byte, error) {
	cmd := conn.GetCmd("sudo", append([]string{"-S"}, args...)...)

	stdin, stdout, _, closer, err := cmd.Interactive()
	if err != nil {
		return nil, fmt.Errorf("failed to start: %w", err)
	}
	defer func() { _ = closer() }()

	payload := []byte(password + "\n")
	n, err := stdin.Write(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}
	if n < len(payload) {
		return nil, fmt.Errorf("short write: wrote %d of %d bytes", n, len(payload))
	}
	stdin.Close()

	out, _ := io.ReadAll(stdout)

	return out, nil
}
