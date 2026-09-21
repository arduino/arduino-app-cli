// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// ParseReadOutput returns the output of the command that reads a remote file.
// It waits for the first byte, so that a failed read is reported here, and not
// at the first Read of the caller. exit must wait for the command end, and
// return its stderr with its error.
func ParseReadOutput(stdout io.Reader, exit func() ([]byte, error)) (io.Reader, error) {
	parseReadError := func(stderr []byte, err error) error {
		if err == nil {
			return nil
		}

		msg := strings.TrimSpace(string(stderr))
		switch {
		case strings.Contains(msg, "No such file or directory"):
			return fmt.Errorf("%w: %s", fs.ErrNotExist, msg)
		case strings.Contains(msg, "Permission denied"):
			return fmt.Errorf("%w: %s", fs.ErrPermission, msg)
		case msg != "":
			return fmt.Errorf("%w: %s", err, msg)
		default:
			return err
		}
	}

	out := bufio.NewReader(stdout)
	if _, err := out.Peek(1); err != nil {
		// No output at all: the read failed, or the file is empty.
		if failure := parseReadError(exit()); failure != nil {
			return nil, failure
		}
		if !errors.Is(err, io.EOF) {
			return nil, err
		}

		// The file is empty, and the wait above closed the command output.
		return bytes.NewReader(nil), nil
	}

	return out, nil
}

func ParseChage(r io.Reader) (bool, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Last password change") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				return false, fmt.Errorf("unexpected output from chage command: %s", line)
			}
			value := strings.TrimSpace(parts[1])
			return value != "password must be changed", nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, fmt.Errorf("unexpected output from chage command")
}

// ParseLsOutput parses the output of the `ls -laQ` command and returns a slice of FileInfo.
func ParseLsOutput(out io.Reader) ([]FileInfo, error) {
	// skip the first line which contains the total size
	r := bufio.NewReader(out)
	if _, err := r.ReadBytes('\n'); err != nil {
		return nil, err
	}

	var files []FileInfo
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Each entry looks like: `<type><perms> ... "<name>"` (or, for
		// symlinks, `... "<name>" -> "<target>"`). Extract the name from
		// between the first pair of double quotes.
		_, after, ok := strings.Cut(string(line), `"`)
		if !ok {
			continue
		}
		name, _, ok := strings.Cut(after, `"`)
		if !ok {
			continue
		}
		if name == "." || name == ".." {
			continue
		}
		files = append(files, FileInfo{
			Name:      name,
			IsDir:     line[0] == 'd',
			IsSymlink: line[0] == 'l',
		})
	}

	return files, nil
}
