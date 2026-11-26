// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package security

import (
	"bytes"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

var (
	packageIndexPath     = paths.New("testdata/package_index.json")
	packageSignaturePath = paths.New("testdata/package_index.json.sig")
	invalidIndexPath     = paths.New("testdata/invalid_file.json")
)

func TestVerifyArduinoDetachedSignature(t *testing.T) {
	res, signer, err := VerifyArduinoDetachedSignature(packageIndexPath, packageSignaturePath)
	require.NoError(t, err)
	require.NotNil(t, signer)
	require.True(t, res)
	require.Equal(t, uint64(0x7baf404c2dfab4ae), signer.PrimaryKey.KeyId)

	res, signer, err = VerifyArduinoDetachedSignature(invalidIndexPath, packageSignaturePath)
	require.False(t, res)
	require.Nil(t, signer)
	require.Error(t, err)
}

func TestVerifyArduinoDetachedSignatureTampered(t *testing.T) {
	data, err := packageIndexPath.ReadFile()
	require.NoError(t, err)
	tampered := paths.New(t.TempDir()).Join("package_index.json")
	require.NoError(t, tampered.WriteFile(bytes.Replace(data, []byte("arduino"), []byte("arduinO"), 1)))

	res, signer, err := VerifyArduinoDetachedSignature(tampered, packageSignaturePath)
	require.False(t, res)
	require.Nil(t, signer)
	require.Error(t, err)
}

func TestVerifyDetachedSignature(t *testing.T) {
	keyPath := paths.New(t.TempDir()).Join("key.gpg")
	require.NoError(t, keyPath.WriteFile(arduinoPublicKey))

	res, signer, err := VerifyDetachedSignature(packageIndexPath, packageSignaturePath, keyPath)
	require.NoError(t, err)
	require.True(t, res)
	require.Equal(t, uint64(0x7baf404c2dfab4ae), signer.PrimaryKey.KeyId)

	res, _, err = VerifyDetachedSignature(packageIndexPath, packageSignaturePath, keyPath.Parent().Join("missing.gpg"))
	require.False(t, res)
	require.Error(t, err)
}
