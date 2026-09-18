// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package adb

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"sync"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

func adbReadFile(a *ADBConnection, path string) (io.ReadCloser, error) {
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "base64", path) // nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("cannot start adb process: %w", err)
	}
	var stderr bytes.Buffer
	cmd.RedirectStderrTo(&stderr)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	decoded := base64.NewDecoder(base64.StdEncoding, output)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	wait := sync.OnceValue(func() error {
		return remote.CmdError(cmd.Wait(), stderr.Bytes())
	})
	r, err := remote.PeekOutput(decoded, wait)
	if err != nil {
		_ = output.Close()
		return nil, err
	}

	return remote.WithCloser{
		Reader: r,
		CloseFun: func() error {
			// An empty file is over already, and Wait closed the pipe.
			_ = output.Close()
			return wait()
		},
	}, nil
}

func adbWriteFile(a *ADBConnection, r io.Reader, pathStr string) error {
	// Create the file with the correct permissions and ownership
	if _, err := a.run("install", "-o", username, "-g", username, "-m", "0644", "/dev/null", pathStr); err != nil {
		return fmt.Errorf("failed to create file %q: %w", pathStr, err)
	}

	// Write the content to the file.
	cmd, err := paths.NewProcess(nil, a.adbPath, "-s", a.host, "shell", "base64", "-d", ">", pathStr) // nolint:gosec
	if err != nil {
		return fmt.Errorf("cannot create write process: %w", err)
	}
	var stderr bytes.Buffer
	cmd.RedirectStderrTo(&stderr)
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
	// Close cmd regardless of errors happening downstream
	defer func() { _ = cmd.Wait() }()

	if _, err := io.Copy(encoder, r); err != nil {
		return fmt.Errorf("failed to write file %q: %w", pathStr, err)
	}
	_ = encoder.Close()
	_ = stdin.Close() // Close the stdin pipe to signal that we're done writing.

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to write file %q: %w", pathStr, remote.CmdError(err, stderr.Bytes()))
	}

	return nil
}
