// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"go.bug.st/f"
)

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

// ParseStatOutput parses the output of the `stat -c "%A %n"`
func ParseStatOutput(out []byte) (FileInfo, error) {
	out = bytes.TrimSpace(out)
	if bytes.HasPrefix(out, []byte("stat: ")) {
		if bytes.Contains(out, []byte("No such file or directory")) {
			return FileInfo{}, os.ErrNotExist
		}
		return FileInfo{}, fmt.Errorf("unexpected stat output: %q", string(out))
	}

	perm, name, ok := strings.Cut(string(out), " ")
	if !ok || len(perm) < 10 {
		return FileInfo{}, fmt.Errorf("unexpected stat output: %q", string(out))
	}
	return FileInfo{
		Name:  path.Base(string(name)),
		IsDir: perm[0] == 'd',
		Mode:  parsePermissions([]byte(perm[:10])), // make sure to pass only permissions bit.
	}, nil
}

// parsePermissions parses a Unix permission string like "-rwxr-xr-x" into an os.FileMode.
func parsePermissions(s []byte) uint32 {
	f.Assert(len(s) == 10, "permission string must be 10 characters long")

	var mode uint32
	bits := []struct {
		char byte
		bit  uint32
	}{
		{s[1], 0400}, {s[2], 0200}, {s[3], 0100},
		{s[4], 0040}, {s[5], 0020}, {s[6], 0010},
		{s[7], 0004}, {s[8], 0002}, {s[9], 0001},
	}
	for _, b := range bits {
		if b.char != '-' {
			mode |= b.bit
		}
	}
	return mode
}

// ParseLsOutput parses the output of the `ls -laQ` command and returns a slice of FileInfo.
func ParseLsOutput(out io.Reader) ([]FileInfo, error) {
	var files []FileInfo
	scanner := bufio.NewScanner(out)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			if strings.HasPrefix(line, "total") {
				continue
			}
		}
		if len(line) == 0 {
			continue
		}
		first_quote := strings.IndexByte(line, '"')
		last_quote := strings.LastIndexByte(line, '"')
		if first_quote < 0 || last_quote <= first_quote {
			continue
		}
		name := line[first_quote+1 : last_quote]
		if name == "." || name == ".." {
			continue
		}
		mode := parsePermissions([]byte(line[:10]))
		files = append(files, FileInfo{
			Name:  name,
			IsDir: line[0] == 'd',
			Mode:  mode,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return files, nil
}
