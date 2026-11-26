// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"bytes"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

// testSigner is a key that replaces the Arduino key in the tests: the index cannot be signed with it.
var testSigner = sync.OnceValues(func() (*openpgp.Entity, error) {
	return openpgp.NewEntity("test", "", "test@example.com", nil)
})

func testSignerKey(t *testing.T) (*openpgp.Entity, []byte) {
	t.Helper()
	entity, err := testSigner()
	require.NoError(t, err)
	var publicKey bytes.Buffer
	require.NoError(t, entity.Serialize(&publicKey))
	return entity, publicKey.Bytes()
}

func sign(t *testing.T, entity *openpgp.Entity, data string) []byte {
	t.Helper()
	var signature bytes.Buffer
	require.NoError(t, openpgp.DetachSign(&signature, entity, strings.NewReader(data), nil))
	return signature.Bytes()
}

// serveSignedIndex serves the archive of the index, with the signature if not nil.
func serveSignedIndex(t *testing.T, index string, signature []byte) *IndexResource {
	t.Helper()
	res := NewSignedIndexResource(mustParseURL(t, serveFile(t, http.StatusOK, makeIndexArchive(t, index, signature), false)))
	_, res.publicKey = testSignerKey(t)
	return res
}

func TestNewSignedIndexResource(t *testing.T) {
	res := NewSignedIndexResource(mustParseURL(t, "https://example.com/path/brick-index.tar.zst"))
	require.Equal(t, "https://example.com/path/brick-index.tar.zst", res.URL.String())
	require.True(t, res.EnforceSignatureVerification)
}

func TestUpdateSignedIndex(t *testing.T) {
	entity, _ := testSignerKey(t)
	dir := paths.New(t.TempDir())
	signature := sign(t, entity, validIndex)

	index, err := UpdateRemoteIndex(t.Context(), DownloadConfig("test"), serveSignedIndex(t, validIndex, signature), dir)
	require.NoError(t, err)
	require.Equal(t, "acme:camera", index.Bricks[0].ID)
	saved, err := dir.Join(RemoteIndexFileName).ReadFile()
	require.NoError(t, err)
	require.Equal(t, validIndex, string(saved))
	savedSignature, err := dir.Join(RemoteIndexSignatureFileName).ReadFile()
	require.NoError(t, err)
	require.Equal(t, signature, savedSignature)

	// An unsigned update removes the signature of the previous index.
	_, err = UpdateRemoteIndex(t.Context(), DownloadConfig("test"), unsigned(serveSignedIndex(t, validIndex, nil).URL), dir)
	require.NoError(t, err)
	require.NoFileExists(t, dir.Join(RemoteIndexSignatureFileName).String())

	// Without verification, the signature in the archive is ignored, even when it is not valid.
	_, err = UpdateRemoteIndex(t.Context(), DownloadConfig("test"), unsigned(serveSignedIndex(t, validIndex, []byte("not a signature")).URL), dir)
	require.NoError(t, err)
	require.NoFileExists(t, dir.Join(RemoteIndexSignatureFileName).String())
}

func TestUpdateRejectsBadSignature(t *testing.T) {
	entity, _ := testSignerKey(t)
	otherIndex := strings.ReplaceAll(validIndex, "acme:camera", "acme:audio")
	otherEntity, err := openpgp.NewEntity("other", "", "other@example.com", nil)
	require.NoError(t, err)

	tests := []struct {
		name    string
		res     func(t *testing.T) *IndexResource
		wantErr string
	}{
		{"missing signature", func(t *testing.T) *IndexResource {
			return serveSignedIndex(t, otherIndex, nil)
		}, "missing signature"},
		{"signature of another file", func(t *testing.T) *IndexResource {
			return serveSignedIndex(t, otherIndex, sign(t, entity, validIndex))
		}, "verifying the signature of the bricks index"},
		{"signature made with another key", func(t *testing.T) *IndexResource {
			return serveSignedIndex(t, otherIndex, sign(t, otherEntity, otherIndex))
		}, "verifying the signature of the bricks index"},
		{"garbage signature", func(t *testing.T) *IndexResource {
			return serveSignedIndex(t, otherIndex, []byte("not a signature"))
		}, "verifying the signature of the bricks index"},
		{"signed with the Arduino key", func(t *testing.T) *IndexResource {
			res := serveSignedIndex(t, otherIndex, sign(t, entity, otherIndex))
			res.publicKey = nil
			return res
		}, "verifying the signature of the bricks index"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := paths.New(t.TempDir())
			require.NoError(t, dir.Join(RemoteIndexFileName).WriteFile([]byte(validIndex)))

			_, err := UpdateRemoteIndex(t.Context(), DownloadConfig("test"), tt.res(t), dir)
			require.ErrorContains(t, err, tt.wantErr)

			// The previous index is kept, and no temporary files are left behind.
			saved, err := dir.Join(RemoteIndexFileName).ReadFile()
			require.NoError(t, err)
			require.Equal(t, validIndex, string(saved))
			entries, err := dir.ReadDir()
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}
