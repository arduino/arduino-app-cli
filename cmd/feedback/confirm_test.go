// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package feedback

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "yes", input: "yes\n", want: true},
		{name: "y", input: "y\n", want: true},
		{name: "upper case", input: "YES\n", want: true},
		{name: "surrounding spaces", input: "  y  \n", want: true},
		{name: "no trailing newline", input: "y", want: true},
		{name: "no", input: "n\n", want: false},
		{name: "empty answer", input: "\n", want: false},
		{name: "more words", input: "yes please\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reset()
			out := new(bytes.Buffer)
			SetOut(out)
			SetFormat(Text)

			got, err := Confirm("Are you sure? (yes/no)", strings.NewReader(tt.input))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, "Are you sure? (yes/no)\n", out.String())
		})
	}

	t.Run("no input", func(t *testing.T) {
		reset()
		SetOut(new(bytes.Buffer))
		SetFormat(Text)

		got, err := Confirm("Are you sure? (yes/no)", strings.NewReader(""))
		require.ErrorIs(t, err, io.EOF)
		require.False(t, got)
	})
}
