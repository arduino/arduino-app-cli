// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

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
