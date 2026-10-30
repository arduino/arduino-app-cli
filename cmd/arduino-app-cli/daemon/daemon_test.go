// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartOutputKeepsTheLastLines(t *testing.T) {
	var output startOutput
	for i := range startOutputLines + 5 {
		output.add(fmt.Sprintf("line %d", i))
	}

	lines := strings.Split(output.String(), "\n")
	require.Len(t, lines, startOutputLines)
	require.Equal(t, "line 5", lines[0])
	require.Equal(t, fmt.Sprintf("line %d", startOutputLines+4), lines[len(lines)-1])
}

func TestStartOutputIsEmptyWithoutLines(t *testing.T) {
	var output startOutput
	require.Empty(t, output.String())
}
