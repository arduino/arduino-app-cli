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

package platform

import (
	"encoding/json"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPlatformWithOverride(t *testing.T) {
	tmpDir := paths.New(t.TempDir())
	override := Platform{
		FQBN: "some:custom:board",
	}

	f, err := tmpDir.Join("platform.json").Create()
	require.NoError(t, err)
	defer f.Close()
	err = json.NewEncoder(f).Encode(override)
	require.NoError(t, err)

	p := GetPlatform(tmpDir)
	assert.Equal(t, "some:custom:board", p.FQBN)
}
