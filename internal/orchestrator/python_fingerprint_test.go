// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestThePythonFingerprintMatchesNothingOnceForgotten(t *testing.T) {
	arduinoApp := app.ArduinoApp{FullPath: paths.New(t.TempDir())}
	python := pythonFingerprint(arduinoApp)
	hash := hashOf([]byte("services: {main: {image: python-apps-base}}"))

	require.NoError(t, python.forget(), "nothing was saved yet")

	require.NoError(t, python.save(hash))
	assert.True(t, python.matches(hash))
	assert.FileExists(t, arduinoApp.ProvisioningStateDir().Join("python", "fingerprint.json").String())

	require.NoError(t, python.forget())
	assert.False(t, python.matches(hash), "a start that failed leaves no record")
}

func TestThePythonHashFollowsTheModelOfEachBrick(t *testing.T) {
	rendered := []byte("services: {main: {image: python-apps-base}}")
	index := &bricksindex.BricksIndex{BuiltInBricks: []bricksindex.Brick{
		{ID: "arduino:llm"},
		{ID: "arduino:object_detection", ModelName: "yolox-object-detection"},
	}}
	hashWith := func(bricks ...app.Brick) string { return pythonHashOf(rendered, bricks, index) }
	llm := func(model string) app.Brick { return app.Brick{ID: "arduino:llm", Model: model} }
	detection := func(model string) app.Brick { return app.Brick{ID: "arduino:object_detection", Model: model} }

	started := hashWith(llm("llamacpp:qwen"), detection(""))

	assert.Equal(t, started, hashWith(llm("llamacpp:qwen"), detection("")), "nothing changed")
	assert.Equal(t, started, hashWith(llm("llamacpp:qwen"), detection("yolox-object-detection")), "the default of the brick, now named")
	assert.NotEqual(t, started, hashWith(llm("llamacpp:gemma"), detection("")), "another model, in the same compose file")
	assert.NotEqual(t, started, hashWith(llm("llamacpp:qwen"), detection("my-model")), "a model in place of the default")
	assert.NotEqual(t, started, hashWith(llm(""), detection("llamacpp:qwen")), "the same model on another brick")
	assert.NotEqual(t, started, pythonHashOf([]byte("services: {}"), []app.Brick{llm("llamacpp:qwen"), detection("")}, index), "another compose file")
}
