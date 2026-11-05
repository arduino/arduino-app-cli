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
	sketch := partFingerprint(cache, "sketch")
	hash := hashOf([]byte("void setup() {}"))

	assert.False(t, sketch.matches(hash), "nothing was built yet")

	binary := cache.Join("sketch", "sketch.ino.bin")
	require.NoError(t, binary.Parent().MkdirAll())
	require.NoError(t, binary.WriteFile([]byte("firmware")))
	require.NoError(t, sketch.save(hash, binary.String()))
	assert.True(t, sketch.matches(hash))
	assert.False(t, sketch.matches(hashOf([]byte("void setup() { }"))), "a changed input needs a build")
	assert.FileExists(t, cache.Join("sketch", "fingerprint.json").String())

	require.NoError(t, binary.Remove())
	assert.False(t, sketch.matches(hash), "a missing output needs a build")

	require.NoError(t, cache.Join("python").MkdirAll())
	require.NoError(t, cache.Join("python", "fingerprint.json").WriteFile([]byte("not json")))
	assert.False(t, partFingerprint(cache, "python").matches(hash), "an unreadable record matches nothing")
}
