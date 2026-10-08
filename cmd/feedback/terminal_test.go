// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package feedback

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsYes(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		want   bool
	}{
		{name: "yes", answer: "yes", want: true},
		{name: "y", answer: "y", want: true},
		{name: "upper case", answer: "YES", want: true},
		{name: "surrounding spaces", answer: "  y  ", want: true},
		{name: "no", answer: "n", want: false},
		{name: "empty answer", answer: "", want: false},
		{name: "more words", answer: "yes please", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isYes(tt.answer))
		})
	}
}
