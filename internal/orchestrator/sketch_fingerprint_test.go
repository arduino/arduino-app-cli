// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSketchInputs(t *testing.T) {
	sketch := paths.New(t.TempDir()).Join("sketch")
	require.NoError(t, sketch.Join("src").MkdirAll())
	ino := sketch.Join("sketch.ino")
	require.NoError(t, ino.WriteFile([]byte("void setup() {}\nvoid loop() {}\n")))
	require.NoError(t, sketch.Join("sketch.yaml").WriteFile([]byte("profiles:\n  default:\n    fqbn: arduino:zephyr:unoq\n")))
	require.NoError(t, sketch.Join("src", "helper.h").WriteFile([]byte("#pragma once\n")))

	const fqbn, platform = "arduino:zephyr:unoq:wait_linux_boot=app", "1.0.0"
	inputs := func() string {
		data, err := sketchInputs(sketch, fqbn, false, platform)
		require.NoError(t, err)
		return hashOf(data)
	}
	same := inputs()

	past := time.Now().Add(-time.Hour)
	require.NoError(t, ino.Chtimes(past, past))
	assert.Equal(t, same, inputs(), "a modification time is not an input")

	require.NoError(t, ino.WriteFile([]byte("void setup() {}\nvoid loop() { delay(1); }\n")))
	edited := inputs()
	assert.NotEqual(t, same, edited, "the sketch code is an input")

	require.NoError(t, sketch.Join("sketch.yaml").WriteFile([]byte("profiles:\n  default:\n    fqbn: arduino:zephyr:unoq\n    libraries:\n      - Servo (1.2.0)\n")))
	assert.NotEqual(t, edited, inputs(), "the profile is an input")

	require.NoError(t, sketch.Join("src", "helper.h").WriteFile([]byte("#pragma once\n#define FAST 1\n")))
	nested := inputs()
	assert.NotEqual(t, edited, nested, "a file in a subfolder is an input")

	for _, variant := range []struct {
		name  string
		input func() ([]byte, error)
	}{
		{"fqbn", func() ([]byte, error) { return sketchInputs(sketch, "arduino:zephyr:unoq", false, platform) }},
		{"flash mode", func() ([]byte, error) { return sketchInputs(sketch, fqbn, true, platform) }},
		{"platform version", func() ([]byte, error) { return sketchInputs(sketch, fqbn, false, "1.1.0") }},
	} {
		data, err := variant.input()
		require.NoError(t, err)
		assert.NotEqual(t, nested, hashOf(data), "the %s is an input", variant.name)
	}
}

func TestBuildArtifacts(t *testing.T) {
	build := paths.New(t.TempDir())
	sketch := build.Join("..", "sketch")
	for _, name := range []string{"sketch.ino.elf", "sketch.ino.elf-zsk.bin", "sketch.ino.map", "includes.cache", "build.options.json"} {
		require.NoError(t, build.Join(name).WriteFile(nil))
	}
	require.NoError(t, build.Join("sketch").MkdirAll())
	require.NoError(t, build.Join("sketch", "sketch.ino.cpp.o").WriteFile(nil))

	assert.ElementsMatch(t, []string{
		build.Join("sketch.ino.elf").String(),
		build.Join("sketch.ino.elf-zsk.bin").String(),
		build.Join("sketch.ino.map").String(),
	}, buildArtifacts(build, sketch))
	assert.Empty(t, buildArtifacts(build.Join("missing"), sketch))
}
