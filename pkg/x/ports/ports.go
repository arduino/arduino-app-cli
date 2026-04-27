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
