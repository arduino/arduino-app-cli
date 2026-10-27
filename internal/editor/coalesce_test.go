// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package editor

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
)

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name string
		in   []rawEvent
		want []changeEvent
	}{
		{
			"create+writes -> create",
			[]rawEvent{
				{Op: fsnotify.Create, Path: paths.New("/root/a")},
				{Op: fsnotify.Write, Path: paths.New("/root/a")},
				{Op: fsnotify.Write, Path: paths.New("/root/a")},
			},
			[]changeEvent{{Type: "create", Path: paths.New("/root/a")}},
		},
		{
			"writes -> update",
			[]rawEvent{
				{Op: fsnotify.Write, Path: paths.New("/root/a")},
				{Op: fsnotify.Write, Path: paths.New("/root/a")},
			},
			[]changeEvent{{Type: "update", Path: paths.New("/root/a")}},
		},
		{
			"create+remove cancels",
			[]rawEvent{
				{Op: fsnotify.Create, Path: paths.New("/root/tmp")},
				{Op: fsnotify.Remove, Path: paths.New("/root/tmp")},
			},
			nil,
		},
		{
			"rename pairing",
			[]rawEvent{
				{Op: fsnotify.Rename, Path: paths.New("/root/a")},
				{Op: fsnotify.Create, Path: paths.New("/root/b")},
			},
			[]changeEvent{{Type: "rename", Path: paths.New("/root/b"), OldPath: paths.New("/root/a")}},
		},
		{
			"dir rename with contents pairs and drops descendants",
			[]rawEvent{
				{Op: fsnotify.Rename, Path: paths.New("/root/py"), IsDir: true},
				{Op: fsnotify.Create, Path: paths.New("/root/py2"), IsDir: true},
				{Op: fsnotify.Create, Path: paths.New("/root/py2/main.py")},
			},
			[]changeEvent{{Type: "rename", Path: paths.New("/root/py2"), OldPath: paths.New("/root/py"), IsDir: true}},
		},
		{
			"unpaired delete+create in different dirs stay separate",
			[]rawEvent{
				{Op: fsnotify.Remove, Path: paths.New("/root/a")},
				{Op: fsnotify.Create, Path: paths.New("/root/sub/b")},
			},
			[]changeEvent{
				{Type: "delete", Path: paths.New("/root/a")},
				{Type: "create", Path: paths.New("/root/sub/b")},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := coalesce(tc.in)
			if tc.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMatchGlobs(t *testing.T) {
	assert.False(t, matchGlobs("node_modules/foo/bar.js", nil, []string{"node_modules/**"}))
	assert.True(t, matchGlobs("src/app.ts", nil, []string{"node_modules/**"}))
	assert.True(t, matchGlobs("src/app.ts", []string{"**/*.ts"}, nil))
	assert.False(t, matchGlobs("src/app.js", []string{"**/*.ts"}, nil))
}
