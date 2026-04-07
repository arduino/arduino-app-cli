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

//go:build windows

package adb

import (
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
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

	return remote.WithCloser{
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
	stdout, err := cmd.RunAndCaptureCombinedOutput(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to create file to %q: %w: %s", pathStr, err, string(stdout))
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
	// Close cmd regardless of errors happening downstream
	defer func() { _ = cmd.Wait() }()

	if _, err := io.Copy(encoder, r); err != nil {
		return fmt.Errorf("failed to write file %q: %w", pathStr, err)
	}
	_ = encoder.Close()
	_ = stdin.Close() // Close the stdin pipe to signal that we're done writing.

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to close command for writing file %q: %w", pathStr, err)
	}
	return nil
}
