// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package feedback

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// Confirm prints the question and reads the answer from in.
func Confirm(question string, in io.Reader) (bool, error) {
	Print(question)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "yes" || answer == "y", nil
}
