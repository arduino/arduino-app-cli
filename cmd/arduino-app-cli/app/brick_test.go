// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBrickVariables(t *testing.T) {
	t.Run("prompts only for variables without a value", func(t *testing.T) {
		var prompted []string
		variables, err := parseBrickVariables([]string{"PROMPT", "VALUE=updated", "CLEAR="}, func(name string) (string, error) {
			prompted = append(prompted, name)
			return "entered", nil
		})

		require.NoError(t, err)
		require.Equal(t, []string{"PROMPT"}, prompted)
		require.Equal(t, map[string]string{
			"PROMPT": "entered",
			"VALUE":  "updated",
			"CLEAR":  "",
		}, variables)
	})

	t.Run("returns input errors", func(t *testing.T) {
		inputErr := errors.New("input unavailable")
		_, err := parseBrickVariables([]string{"PROMPT"}, func(string) (string, error) {
			return "", inputErr
		})

		require.ErrorIs(t, err, inputErr)
	})
}
