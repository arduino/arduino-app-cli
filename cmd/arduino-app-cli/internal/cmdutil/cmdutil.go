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
