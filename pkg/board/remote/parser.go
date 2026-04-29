// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote

import (
	"bufio"
	"fmt"
	"io"
	"strings"
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
