// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestListInstalled(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	write := func(folder, config string) {
		dir := bricksDir.Join(folder)
		require.NoError(t, dir.MkdirAll())
		if config != "" {
			require.NoError(t, dir.Join(brickConfigFileName).WriteFile([]byte(config)))
		}
	}
	write("acme/camera/1.10.0", "id: acme:camera\nname: Camera\n")
	write("acme/camera/1.2.0", "id: acme:camera\nname: Camera\n")
	write("acme/audio/1.0.0", "id: acme:audio\nname: Audio\n")
	write("acme/wrong/1.0.0", "id: acme:other\n")         // declares another id
	write("acme/missing/1.0.0", "")                       // no brick_config.yaml
	write("acme/camera/notes", "id: acme:camera\n")       // not created by install
	write("acme/camera@1.0.0", "id: acme:camera\n")       // old layout
	write("acme/Camera/1.0.0", "id: acme:Camera\n")       // invalid name
	write(".install-123/x/1.0.0", "id: .install-123:x\n") // temporary folder

	installed, err := ListInstalled(bricksDir)
	require.NoError(t, err)

	got := make([]string, 0, len(installed))
	for _, b := range installed {
		require.Equal(t, "acme", b.Namespace)
		got = append(got, b.Brick.ID+"@"+b.Version+" "+b.Brick.Name)
	}
	require.Equal(t, []string{
		"acme:audio@1.0.0 Audio",
		"acme:camera@1.2.0 Camera",
		"acme:camera@1.10.0 Camera",
	}, got)

	installed, err = ListInstalled(bricksDir.Join("missing"))
	require.NoError(t, err)
	require.Empty(t, installed)
}
