// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package adb

import (
	"bytes"
	"cmp"
	"fmt"
	"io"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

func adbReadFile(a *ADBConnection, path string) (io.ReadCloser, error) {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "cat", path) // nolint:gosec
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
	r, err := remote.PeekOutput(output, func() error {
		return remote.CmdError(cmd.Wait(), stderr.Bytes())
	})
	if err != nil {
		_ = output.Close()
		return nil, err
	}

	return remote.WithCloser{
		Reader: r,
		CloseFun: func() error {
			return cmp.Or(output.Close(), cmd.Wait())
		},
	}, nil
}

func adbWriteFile(a *ADBConnection, r io.Reader, pathStr string) error {
	// Create the file with the correct permissions and ownership
	if _, err := a.run("install", "-o", username, "-g", username, "-m", "0644", "/dev/null", pathStr); err != nil {
		return fmt.Errorf("failed to create file %q: %w", pathStr, err)
	}

	// Write the content to the file.
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "cat", ">", pathStr) // nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to create command to write file %q: %w", pathStr, err)
	}
	var stderr bytes.Buffer
	cmd.RedirectStderrTo(&stderr)
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
		return fmt.Errorf("failed to write file %q: %w", pathStr, remote.CmdError(err, stderr.Bytes()))
	}

	return nil
}
