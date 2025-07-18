//go:build !windows

package adb

import (
	"context"
	"fmt"
	"io"

	"github.com/arduino/go-paths-helper"
)

func adbReadFile(a *ADBConnection, path string) (io.ReadCloser, error) {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "cat", path) // nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to create command to read file %q: %w", path, err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return output, nil
}

func adbWriteFile(a *ADBConnection, r io.Reader, pathStr string) error {
	// Create the file with the correct permissions and ownership
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "install", "-o", username, "-g", username, "-m", "0644", "/dev/null", pathStr) // nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to create command for creating file %q: %w", pathStr, err)
	}
	stdout, stderr, err := cmd.RunAndCaptureOutput(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to start command for creating file %q: %w: %s%s", pathStr, err, string(stdout), string(stderr))
	}

	// Write the content to the file.
	cmd, err = paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "cat", ">", pathStr) // nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to create command to write file %q: %w", pathStr, err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe for command to write file %q: %w", pathStr, err)
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command for write file %q: %w", pathStr, err)
	}

	if _, err := io.Copy(stdin, r); err != nil {
		return fmt.Errorf("failed to write content to file %q: %w", pathStr, err)
	}
	_ = stdin.Close() // Close the stdin pipe to signal that we're done writing.

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to close command for writing file %q: %w", pathStr, err)
	}
	return nil
}
