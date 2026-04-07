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

package ports

import (
	"fmt"
	"math/rand/v2"
	"net"
)

const forwardPortAttempts = 10

func GetAvailable() (int, error) {
	tried := make(map[int]any, forwardPortAttempts)
	for len(tried) < forwardPortAttempts {
		port := getRandomPort()
		if _, seen := tried[port]; seen {
			continue
		}
		tried[port] = struct{}{}

		if IsAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range 1000-9999 after %d attempts", forwardPortAttempts)
}

func IsAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

func getRandomPort() int {
	port := 1000 + rand.IntN(9000) // nolint:gosec
	return port
}
