// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package editor

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
	"unicode/utf8"

	"github.com/arduino/go-paths-helper"
	"github.com/sourcegraph/jsonrpc2"
)

// sniffWindow bounds the UTF-8/NUL sniff that picks an "auto" fs.read
// encoding, per editor-service-reference.md's "first ~512 bytes".
const sniffWindow = 512

var errNotUTF8 = errors.New("content is not valid utf-8")

type statParams struct {
	Path string `json:"path"`
}

type statResult struct {
	Size  int64     `json:"size"`
	MTime time.Time `json:"mtime"`
	IsDir bool      `json:"isDir"`
	ETag  string    `json:"etag"`
}

type readParams struct {
	Path     string  `json:"path"`
	Offset   int64   `json:"offset,omitempty"`
	Length   int64   `json:"length,omitempty"`
	BaseETag *string `json:"baseEtag,omitempty"`
	Encoding string  `json:"encoding,omitempty"`
}

type readResult struct {
	Encoding    string `json:"encoding"`
	Content     string `json:"content"`
	ETag        string `json:"etag"`
	Size        int64  `json:"size"`
	NotModified bool   `json:"-"`
}

func (r readResult) MarshalJSON() ([]byte, error) {
	if r.NotModified {
		return json.Marshal(struct {
			NotModified bool   `json:"notModified"`
			ETag        string `json:"etag"`
		}{true, r.ETag})
	}
	type plain readResult
	return json.Marshal(plain(r))
}

type writeParams struct {
	Path     string  `json:"path"`
	Encoding string  `json:"encoding"`
	Content  string  `json:"content"`
	BaseETag *string `json:"baseEtag,omitempty"`
	Atomic   bool    `json:"atomic,omitempty"`
}

type writeResult struct {
	ETag string `json:"etag"`
	Size int64  `json:"size"`
}

type mkdirParams struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

type deleteParams struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

// requireAbsPath validates path the way fs.watch and fs.walk already do
// inline; the five fs.* methods below share it since each needs the check
// exactly once, unlike those two.
func requireAbsPath(raw string) (*paths.Path, *jsonrpc2.Error) {
	if raw == "" {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "path is required"}
	}
	p := paths.New(raw)
	if !p.IsAbs() {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: "path must be absolute"}
	}
	return p, nil
}

// etagOf is built from mtime and size, not the impl doc's ctime-including
// version: that needs syscall.Stat_t, whose shape is not the same on every OS
// this module is built on (the board is always linux, but `go test` also runs
// on the dev's own host). ext4's nanosecond mtime is what the doc says makes
// ctime unnecessary in practice; the pathological same-second, same-size
// delete+recreate case is what fs.hash is for.
func etagOf(info os.FileInfo) string {
	return fmt.Sprintf("%x-%x", info.ModTime().UnixNano(), info.Size())
}

// statSize is 0 for directories, matching FileEntry's convention in fs.walk.
func statSize(info os.FileInfo) int64 {
	if info.IsDir() {
		return 0
	}
	return info.Size()
}

func statPath(target *paths.Path) (statResult, *jsonrpc2.Error) {
	info, err := target.Stat()
	if err != nil {
		return statResult{}, classifyFSError(err)
	}
	return statResult{
		Size:  statSize(info),
		MTime: info.ModTime(),
		IsDir: info.IsDir(),
		ETag:  etagOf(info),
	}, nil
}

// sliceRange applies a fs.read offset/length to data. An offset at or beyond
// the end yields an empty slice rather than an error: the client already has
// the full size from the result and can tell the difference.
func sliceRange(data []byte, offset, length int64) []byte {
	if offset <= 0 && length <= 0 {
		return data
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= int64(len(data)) {
		return nil
	}
	end := int64(len(data))
	if length > 0 && offset+length < end {
		end = offset + length
	}
	return data[offset:end]
}

func readPath(target *paths.Path, p readParams) (readResult, *jsonrpc2.Error) {
	info, err := target.Stat()
	if err != nil {
		return readResult{}, classifyFSError(err)
	}
	etag := etagOf(info)
	if p.BaseETag != nil && *p.BaseETag == etag {
		return readResult{ETag: etag, NotModified: true}, nil
	}

	data, err := target.ReadFile()
	if err != nil {
		return readResult{}, classifyFSError(err)
	}
	data = sliceRange(data, p.Offset, p.Length)

	encoding, content, err := encodeContent(data, p.Encoding)
	if err != nil {
		jerr := &jsonrpc2.Error{Code: codeEncodingError, Message: err.Error()}
		jerr.SetError(map[string]string{"path": target.String()})
		return readResult{}, jerr
	}
	return readResult{Encoding: encoding, Content: content, ETag: etag, Size: statSize(info)}, nil
}

func writePath(target *paths.Path, p writeParams) (writeResult, *jsonrpc2.Error) {
	if p.BaseETag != nil {
		current := ""
		if info, err := target.Stat(); err == nil {
			current = etagOf(info)
		} else if !errors.Is(err, os.ErrNotExist) {
			return writeResult{}, classifyFSError(err)
		}
		if *p.BaseETag != current {
			jerr := &jsonrpc2.Error{Code: codeEtagMismatch, Message: "etag mismatch"}
			jerr.SetError(map[string]string{"currentEtag": current})
			return writeResult{}, jerr
		}
	}

	data, err := decodeContent(p.Encoding, p.Content)
	if err != nil {
		return writeResult{}, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams, Message: err.Error()}
	}

	if p.Atomic {
		// Same directory as target, so the rename below is same-filesystem and
		// therefore atomic rather than a copy.
		tmp, err := paths.WriteToTempFile(data, target.Parent(), "editor-write-*")
		if err != nil {
			return writeResult{}, classifyFSError(err)
		}
		// os.CreateTemp always mints 0600; match WriteFile's own 0644 so an
		// atomic write is not silently less readable than a plain one.
		if err := tmp.Chmod(0o644); err != nil {
			_ = tmp.Remove()
			return writeResult{}, classifyFSError(err)
		}
		if err := tmp.Rename(target); err != nil {
			_ = tmp.Remove()
			return writeResult{}, classifyFSError(err)
		}
	} else if err := target.WriteFile(data); err != nil {
		return writeResult{}, classifyFSError(err)
	}

	info, err := target.Stat()
	if err != nil {
		return writeResult{}, classifyFSError(err)
	}
	return writeResult{ETag: etagOf(info), Size: statSize(info)}, nil
}

func mkdirPath(target *paths.Path, recursive bool) *jsonrpc2.Error {
	var err error
	if recursive {
		err = target.MkdirAll()
	} else {
		err = target.Mkdir()
	}
	if err != nil {
		return classifyFSError(err)
	}
	return nil
}

func deletePath(target *paths.Path, recursive bool) *jsonrpc2.Error {
	var err error
	if recursive {
		err = target.RemoveAll()
	} else {
		err = target.Remove()
	}
	if err != nil {
		return classifyFSError(err)
	}
	return nil
}

// decodeContent turns a fs.write payload into bytes per its encoding.
func decodeContent(encoding, content string) ([]byte, error) {
	switch encoding {
	case "utf-8":
		return []byte(content), nil
	case "base64":
		return base64.StdEncoding.DecodeString(content)
	default:
		return nil, fmt.Errorf("unsupported encoding: %q", encoding)
	}
}

// encodeContent picks the fs.read response encoding: the caller's choice, or
// a sniff of the first sniffWindow bytes when it asks for "auto" (the
// default). "utf-8" forced on non-UTF-8 bytes is an encodingError, never a
// silent fallback to base64.
func encodeContent(data []byte, requested string) (encoding, content string, err error) {
	switch requested {
	case "base64":
		return "base64", base64.StdEncoding.EncodeToString(data), nil
	case "utf-8":
		if !utf8.Valid(data) {
			return "", "", errNotUTF8
		}
		return "utf-8", string(data), nil
	case "", "auto":
		if looksLikeText(data) {
			return "utf-8", string(data), nil
		}
		return "base64", base64.StdEncoding.EncodeToString(data), nil
	default:
		return "", "", fmt.Errorf("unsupported encoding: %q", requested)
	}
}

func looksLikeText(data []byte) bool {
	head := data
	if len(head) > sniffWindow {
		head = head[:sniffWindow]
	}
	return utf8.Valid(head) && !bytes.Contains(head, []byte{0})
}
