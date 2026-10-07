// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package feedback

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/arduino/arduino-app-cli/cmd/i18n"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

// IsInteractive returns true if the CLI is interactive (it can receive inputs from terminal/console)
func IsInteractive() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
}

// InputUserField prompts the user to input the provided user field.
func InputUserField(prompt string, secret bool) (string, error) {
	if format != Text {
		return "", errors.New(i18n.Tr("user input not supported for the '%s' output format", format))
	}
	if !IsInteractive() {
		return "", errors.New(i18n.Tr("user input not supported in non interactive mode"))
	}

	fmt.Fprintf(stdOut, "%s: ", prompt)

	if secret {
		// Read and return a password (no characters echoed on terminal)
		value, err := term.ReadPassword(int(os.Stdin.Fd())) // nolint: gosec
		fmt.Fprintln(stdOut)
		return string(value), err
	}

	// Read and return an input line
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return sc.Text(), nil
}

func Confirm(question string) (bool, error) {
	answer, err := InputUserField(question, false)
	if err != nil {
		return false, err
	}
	return isYes(answer), nil
}

func isYes(answer string) bool {
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "yes" || answer == "y"
}
