// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package cmdutil

import (
	"encoding/base64"
	"strings"

	"github.com/arduino/go-paths-helper"
	"golang.org/x/term"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

// IDToAlias returns the string representation of an app ID in a readable and short way.
// Either with the id itself or a relative path if possible.
func IDToAlias(id app.ID) string {
	v := id.String()
	res, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return v
	}

	v = string(res)
	if strings.Contains(v, ":") {
		return v
	}

	wd, err := paths.Getwd()
	if err != nil {
		return v
	}
	rel, err := paths.New(v).RelFrom(wd)
	if err != nil {
		return v
	}
	if !strings.HasPrefix(rel.String(), "./") && !strings.HasPrefix(rel.String(), "../") {
		return "./" + rel.String()
	}
	return rel.String()
}

func AskForPassword() (string, error) {
	feedback.Printf("Password: ")
	bytePassword, err := term.ReadPassword(int(feedback.GetStdin().Fd())) // nolint:gosec
	if err != nil {
		return "", err
	}
	return string(bytePassword), nil
}
