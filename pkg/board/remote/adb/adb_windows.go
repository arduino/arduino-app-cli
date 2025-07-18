//go:build windows

package adb

import (
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
)

func adbReadFile(a *ADBConnection, path string) (io.ReadCloser, error) {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "base64", path) // nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("cannot start adb process: %w", err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	decoded := base64.NewDecoder(base64.StdEncoding, output)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return ssh.WithCloser{
		Reader: decoded,
		CloseFun: func() error {
			err1 := output.Close()
			err2 := cmd.Wait()
			return cmp.Or(err1, err2)
		},
	}, nil
}

func adbWriteFile(a *ADBConnection, r io.Reader, pathStr string) error {
	// Create the file with the correct permissions and ownership
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "install", "-o", username, "-g", username, "-m", "0644", "/dev/null", pathStr) // nolint:gosec
	if err != nil {
		return fmt.Errorf("cannot create process: %w", err)
	}
	stdout, stderr, err := cmd.RunAndCaptureOutput(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to create file to %q: %w: %s: %s", pathStr, err, string(stdout), string(stderr))
	}

	// Write the content to the file.
	cmd, err = paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "base64", "-d", ">", pathStr) // nolint:gosec
	if err != nil {
		return fmt.Errorf("cannot create write process: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("cannot create stdin pipe: %w", err)
	}
	defer stdin.Close()

	encoder := base64.NewEncoder(base64.StdEncoding, stdin)
	defer encoder.Close()

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start write process %q: %w", pathStr, err)
	}

	if _, err := io.Copy(encoder, r); err != nil {
		return fmt.Errorf("failed to write file %q: %w", pathStr, err)
	}
	encoder.Close()
	stdin.Close()

	_ = cmd.Wait()

	return nil
}
