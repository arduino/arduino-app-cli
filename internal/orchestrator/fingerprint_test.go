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
)

func TestFingerprint(t *testing.T) {
	cache := paths.New(t.TempDir())
	python := partFingerprint(cache, "python")
	hash := hashOf([]byte("services: {}"))

	assert.False(t, python.matches(hash), "nothing was built yet")

	require.NoError(t, python.save(hash))
	assert.True(t, python.matches(hash))
	assert.False(t, python.matches(hashOf([]byte("services: {main: {}}"))), "a changed input needs a build")
	assert.FileExists(t, cache.Join("python", "fingerprint.json").String())

	require.NoError(t, cache.Join("sketch").MkdirAll())
	require.NoError(t, cache.Join("sketch", "fingerprint.json").WriteFile([]byte("not json")))
	assert.False(t, partFingerprint(cache, "sketch").matches(hash), "an unreadable record matches nothing")
}
