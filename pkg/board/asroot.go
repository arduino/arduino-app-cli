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

// ExecAsRoot runs args through sudo, feeding password on stdin. It returns the
// output of the command, and an error if the command did not exit successfully:
// a wrong password is reported as a failure like any other, since sudo exits with
// the same status in both cases.
func ExecAsRoot(conn remote.RemoteConn, password string, args ...string) ([]byte, error) {
	// -p '' keeps the password prompt of sudo out of the captured output.
	cmd := conn.GetCmd("sudo", append([]string{"-S", "-p", ""}, args...)...)

	stdin, stdout, stderr, closer, err := cmd.Interactive()
	if err != nil {
		return nil, fmt.Errorf("failed to start: %w", err)
	}

	payload := []byte(password + "\n")
	n, err := stdin.Write(payload)
	if err != nil {
		_ = closer()
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}
	if n < len(payload) {
		_ = closer()
		return nil, fmt.Errorf("short write: wrote %d of %d bytes", n, len(payload))
	}
	stdin.Close()

	// sudo reports a wrong password on stderr, so both streams are collected.
	out, _ := io.ReadAll(io.MultiReader(stdout, stderr))

	// Get the exit status from the closer. This must be done after all other io reads are over.
	if err := closer(); err != nil {
		return out, err
	}

	return out, nil
}
