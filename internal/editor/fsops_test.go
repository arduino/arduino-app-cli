// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package editor

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSliceRange(t *testing.T) {
	data := []byte("0123456789")
	assert.Equal(t, data, sliceRange(data, 0, 0), "no offset/length returns the whole slice")
	assert.Equal(t, []byte("3456789"), sliceRange(data, 3, 0))
	assert.Equal(t, []byte("345"), sliceRange(data, 3, 3))
	assert.Equal(t, []byte("789"), sliceRange(data, 7, 100), "length past the end clamps, does not error")
	assert.Nil(t, sliceRange(data, 100, 0), "offset past the end is empty, not an error")
}

func TestEncodeContent(t *testing.T) {
	text := []byte("package main\n")
	binary := []byte{0x00, 0x01, 0x02, 0xff}

	enc, content, err := encodeContent(text, "auto")
	require.NoError(t, err)
	assert.Equal(t, "utf-8", enc)
	assert.Equal(t, string(text), content)

	enc, content, err = encodeContent(binary, "auto")
	require.NoError(t, err)
	assert.Equal(t, "base64", enc)
	assert.Equal(t, base64.StdEncoding.EncodeToString(binary), content)

	_, _, err = encodeContent(binary, "utf-8")
	assert.ErrorIs(t, err, errNotUTF8, "utf-8 forced on non-UTF-8 bytes must fail, not silently fall back")

	enc, content, err = encodeContent(binary, "base64")
	require.NoError(t, err)
	assert.Equal(t, "base64", enc)
	assert.Equal(t, base64.StdEncoding.EncodeToString(binary), content)
}

func TestDecodeContent(t *testing.T) {
	data, err := decodeContent("utf-8", "hello")
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), data)

	data, err = decodeContent("base64", base64.StdEncoding.EncodeToString([]byte{1, 2, 3}))
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, data)

	_, err = decodeContent("nope", "x")
	assert.Error(t, err)
}

func TestEtagOf_StableAcrossReads(t *testing.T) {
	dir := t.TempDir()
	p := paths.New(filepath.Join(dir, "f.txt"))
	require.NoError(t, p.WriteFile([]byte("v1")))

	info1, err := p.Stat()
	require.NoError(t, err)
	info2, err := p.Stat()
	require.NoError(t, err)
	assert.Equal(t, etagOf(info1), etagOf(info2), "two stats of an unchanged file must agree")
}

func TestStatPath(t *testing.T) {
	dir := t.TempDir()
	file := paths.New(filepath.Join(dir, "f.txt"))
	require.NoError(t, file.WriteFile([]byte("hello")))

	res, jerr := statPath(file)
	require.Nil(t, jerr)
	assert.Equal(t, int64(5), res.Size)
	assert.False(t, res.IsDir)
	assert.NotEmpty(t, res.ETag)

	dres, jerr := statPath(paths.New(dir))
	require.Nil(t, jerr)
	assert.True(t, dres.IsDir)
	assert.Equal(t, int64(0), dres.Size, "directories report size 0, matching fs.walk")

	_, jerr = statPath(paths.New(dir, "missing"))
	require.NotNil(t, jerr)
	assert.Equal(t, int64(codeNotFound), jerr.Code)
}

func TestReadPath_HonoursBaseEtagAndRange(t *testing.T) {
	dir := t.TempDir()
	file := paths.New(filepath.Join(dir, "f.txt"))
	require.NoError(t, file.WriteFile([]byte("0123456789")))

	full, jerr := readPath(file, readParams{})
	require.Nil(t, jerr)
	assert.Equal(t, "utf-8", full.Encoding)
	assert.Equal(t, "0123456789", full.Content)
	assert.Equal(t, int64(10), full.Size)

	ranged, jerr := readPath(file, readParams{Offset: 2, Length: 3})
	require.Nil(t, jerr)
	assert.Equal(t, "234", ranged.Content)
	assert.Equal(t, int64(10), ranged.Size, "size reports the whole file, not the slice")

	same := full.ETag
	notModified, jerr := readPath(file, readParams{BaseETag: &same})
	require.Nil(t, jerr)
	assert.True(t, notModified.NotModified)
	assert.Empty(t, notModified.Content)

	stale := "not-the-real-etag"
	changed, jerr := readPath(file, readParams{BaseETag: &stale})
	require.Nil(t, jerr)
	assert.False(t, changed.NotModified)
	assert.Equal(t, "0123456789", changed.Content)
}

func TestWritePath_CreatesAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	file := paths.New(filepath.Join(dir, "f.txt"))

	res, jerr := writePath(file, writeParams{Encoding: "utf-8", Content: "v1"})
	require.Nil(t, jerr)
	assert.Equal(t, int64(2), res.Size)
	got, err := file.ReadFile()
	require.NoError(t, err)
	assert.Equal(t, "v1", string(got))

	res2, jerr := writePath(file, writeParams{Encoding: "utf-8", Content: "v2-longer"})
	require.Nil(t, jerr)
	assert.NotEqual(t, res.ETag, res2.ETag)
	got, err = file.ReadFile()
	require.NoError(t, err)
	assert.Equal(t, "v2-longer", string(got))
}

func TestWritePath_AtomicMatchesPlainPermissions(t *testing.T) {
	dir := t.TempDir()
	plain := paths.New(filepath.Join(dir, "plain.txt"))
	atomic := paths.New(filepath.Join(dir, "atomic.txt"))

	_, jerr := writePath(plain, writeParams{Encoding: "utf-8", Content: "x"})
	require.Nil(t, jerr)
	_, jerr = writePath(atomic, writeParams{Encoding: "utf-8", Content: "x", Atomic: true})
	require.Nil(t, jerr)

	got, err := atomic.ReadFile()
	require.NoError(t, err)
	assert.Equal(t, "x", string(got))

	plainInfo, err := plain.Stat()
	require.NoError(t, err)
	atomicInfo, err := atomic.Stat()
	require.NoError(t, err)
	assert.Equal(t, plainInfo.Mode(), atomicInfo.Mode(),
		"an atomic write must not end up less readable than a plain one (os.CreateTemp mints 0600)")
}

func TestWritePath_RejectsAStaleBaseEtag(t *testing.T) {
	dir := t.TempDir()
	file := paths.New(filepath.Join(dir, "f.txt"))
	require.NoError(t, file.WriteFile([]byte("v1")))

	stale := "stale"
	_, jerr := writePath(file, writeParams{Encoding: "utf-8", Content: "v2", BaseETag: &stale})
	require.NotNil(t, jerr)
	assert.Equal(t, int64(codeEtagMismatch), jerr.Code)

	info, err := file.Stat()
	require.NoError(t, err)
	current := etagOf(info)
	res, jerr := writePath(file, writeParams{Encoding: "utf-8", Content: "v2", BaseETag: &current})
	require.Nil(t, jerr)
	assert.NotEmpty(t, res.ETag)
}

func TestMkdirAndDeletePath(t *testing.T) {
	dir := t.TempDir()
	nested := paths.New(filepath.Join(dir, "a", "b"))

	require.NotNil(t, mkdirPath(nested, false), "non-recursive mkdir must fail when the parent is missing")
	require.Nil(t, mkdirPath(nested, true))
	assert.True(t, nested.IsDir())

	require.Nil(t, mkdirPath(nested, true), "recursive mkdir on an existing dir is idempotent")

	file := nested.Join("f.txt")
	require.NoError(t, file.WriteFile([]byte("x")))

	require.NotNil(t, deletePath(nested, false), "non-recursive delete must fail on a non-empty dir")
	require.Nil(t, deletePath(nested, true))
	assert.False(t, nested.Exist())
}

func TestClassifyFSErrorMapsNotFoundAndPermission(t *testing.T) {
	_, err := os.Stat(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	jerr := classifyFSError(err)
	assert.Equal(t, int64(codeNotFound), jerr.Code)
}
