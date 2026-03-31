// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package board

import (
	"fmt"
	"io"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

func ExecAsRoot(conn remote.RemoteConn, password string, args ...string) ([]byte, error) {
	cmd := conn.GetCmd("sudo", append([]string{"-S"}, args...)...)

	stdin, stdout, stderr, closer, err := cmd.Interactive()
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
	errOut, _ := io.ReadAll(stderr)

	if err := closer(); err != nil {
		return nil, fmt.Errorf("sudo failed: %w: %s: %s", err, string(out), string(errOut))
	}

	return out, nil
}
