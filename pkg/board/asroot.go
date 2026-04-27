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
