// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package adb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

func adbReadFile(a *ADBConnection, path string) (io.ReadCloser, error) {
	// LC_ALL=C: the parser reads the failure of the command in English.
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "LC_ALL=C", "cat", path) // nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to create command to read file %q: %w", path, err)
	}
	var stderr bytes.Buffer
	cmd.RedirectStderrTo(&stderr)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// Wait is not idempotent, and both the parser and Close ask for the end.
	exit := sync.OnceValues(func() ([]byte, error) {
		// Wait first: it ends the copy of stderr.
		err := cmd.Wait()
		return stderr.Bytes(), err
	})
	r, err := remote.ParseReadOutput(output, exit)
	if err != nil {
		_ = output.Close()
		return nil, err
	}

	return remote.WithCloser{
		Reader: r,
		CloseFun: func() error {
			// The read reports the command failure; this wait only releases
			// the process, which fails when the close breaks its pipe.
			_ = output.Close()
			_, _ = exit()
			return nil
		},
	}, nil
}

func adbWriteFile(a *ADBConnection, r io.Reader, pathStr string) error {
	// Create the file with the correct permissions and ownership
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "install", "-o", username, "-g", username, "-m", "0644", "/dev/null", pathStr) // nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to create command for creating file %q: %w", pathStr, err)
	}
	stdout, err := cmd.RunAndCaptureCombinedOutput(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to start command for creating file %q: %w: %s", pathStr, err, string(stdout))
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
	// Close cmd regardless of errors happening downstream
	defer func() { _ = cmd.Wait() }()

	if _, err := io.Copy(stdin, r); err != nil {
		return fmt.Errorf("failed to write content to file %q: %w", pathStr, err)
	}
	_ = stdin.Close() // Close the stdin pipe to signal that we're done writing.

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to close command for writing file %q: %w", pathStr, err)
	}
	return nil
}
