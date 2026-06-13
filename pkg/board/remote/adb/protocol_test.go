// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package adb

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteProtocol(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeProtocolRequest(&buf, "host:version"))
		// 4 ASCII hex digits (lowercase) for "host:version" = 12 bytes.
		assert.Equal(t, "000chost:version", buf.String())
	})

	t.Run("empty payload", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeProtocolRequest(&buf, ""))
		assert.Equal(t, "0000", buf.String())
	})

	t.Run("payload too long", func(t *testing.T) {
		// The adb protocol caps each frame at 0xFFFF bytes.
		var buf bytes.Buffer
		err := writeProtocolRequest(&buf, strings.Repeat("a", 0x10000))
		assert.Error(t, err)
	})
}

func TestReadProtocol(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		got, err := readProtocolString(strings.NewReader("0005hello"))
		require.NoError(t, err)
		assert.Equal(t, "hello", got)
	})

	t.Run("empty string", func(t *testing.T) {
		got, err := readProtocolString(strings.NewReader("0000"))
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("string with invalid header", func(t *testing.T) {
		// "zzzz" is not a valid hex length.
		_, err := readProtocolString(strings.NewReader("zzzz"))
		assert.Error(t, err)
	})

	t.Run("status OK", func(t *testing.T) {
		assert.NoError(t, readProtocolStatus(strings.NewReader("OKAY")))
	})

	t.Run("status FAIL", func(t *testing.T) {
		// FAIL is followed by a length-prefixed error: "device offline" = 14 bytes => 0x000e.
		err := readProtocolStatus(strings.NewReader("FAIL000edevice offline"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device offline")
	})

	t.Run("status unexpected", func(t *testing.T) {
		err := readProtocolStatus(strings.NewReader("WHAT"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected adb status")
	})
}
