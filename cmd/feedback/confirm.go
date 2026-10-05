// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package feedback

import (
	"fmt"
	"io"
	"strings"
)

// Confirm prints the question and reads the answer from in.
func Confirm(question string, in io.Reader) (bool, error) {
	Print(question)
	var answer string
	if _, err := fmt.Fscanf(in, "%s\n", &answer); err != nil {
		return false, err
	}
	answer = strings.ToLower(answer)
	return answer == "yes" || answer == "y", nil
}
