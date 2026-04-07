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

package board

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsurePlatformInstalled(t *testing.T) {
	// We skip it in CI, as downloading andinstalling the core takes ~6 minutes
	if os.Getenv("CI") != "" {
		t.Skip("Skipping slow test")
	}
	// Example test function
	err := EnsurePlatformInstalled(t.Context(), "arduino:zephyr:unoq")
	require.NoError(t, err)
}
