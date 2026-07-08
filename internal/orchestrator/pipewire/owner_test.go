// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package pipewire

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnerMarkerLifecycle(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	const uid = 1000

	owned, err := isOwned(uid)
	require.NoError(t, err)
	require.False(t, owned, "should not be owned before markOwned is called")

	require.NoError(t, markOwned(uid))

	owned, err = isOwned(uid)
	require.NoError(t, err)
	require.True(t, owned, "should be owned after markOwned")

	require.NoError(t, clearOwned(uid))

	owned, err = isOwned(uid)
	require.NoError(t, err)
	require.False(t, owned, "should not be owned after clearOwned")
}

func TestClearOwnedIsIdempotent(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	const uid = 1000

	require.NoError(t, clearOwned(uid), "clearing an unowned marker should not error")
	require.NoError(t, markOwned(uid))
	require.NoError(t, clearOwned(uid))
	require.NoError(t, clearOwned(uid), "clearing an already-cleared marker should not error")
}

func TestOwnerMarkerIsolatedPerUID(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	const uidA, uidB = 1000, 1001

	require.NoError(t, markOwned(uidA))

	ownedA, err := isOwned(uidA)
	require.NoError(t, err)
	require.True(t, ownedA)

	ownedB, err := isOwned(uidB)
	require.NoError(t, err)
	require.False(t, ownedB, "marking one uid must not mark another")
}
