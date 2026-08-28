// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package modelid parses the two kinds of string the models API accepts where a model is
// named: an ID, which names a model, and a Source, which asks for one to be fetched.
//
// The two overlap, and nothing here tells them apart. "llamacpp:unsloth/repo/file-Q4_0"
// is a well-formed id and also a well-formed two-field source key. Only models-list.yaml
// decides, so a caller looks the catalog up first and parses a source only when that
// misses. Nothing in this package says where a model comes from, either: that is a
// property of the model, not of its name.
//
// Validation is deliberately loose. It rejects what cannot work - an empty name, a
// traversing segment, a second colon - and accepts everything the shipped models-list.yaml
// declares, including the two thirds of entries that carry no namespace at all.
package modelid

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// ID names an installed or installable model: an optional framework namespace and a name.
//
//	piper-tts-en                              bare, no namespace
//	ei:efficientnet-b4                        namespaced
//	llamacpp:gemma-3-1b-it-Q4_0               a name models-list.yaml declares
//	llamacpp:unsloth/repo/gemma-3-1b-it-Q4_0  a name derived from where the file landed
//
// A name may hold slashes: a model no entry declares is named by the repository path it
// was downloaded from, and every segment belongs to the name. It is what the handler
// listing reports, and the models.ini section llama-server serves the model under.
type ID struct {
	namespace string
	name      string
}

// Parse reads an id, rejecting a shape that could not name a model.
func Parse(s string) (ID, error) {
	if s == "" {
		return ID{}, fmt.Errorf("%w: empty", ErrInvalidID)
	}
	if strings.TrimSpace(s) != s || strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || r == '"' || r == '\\'
	}) {
		return ID{}, fmt.Errorf("%w: %q holds whitespace or a character an id cannot carry", ErrInvalidID, s)
	}

	namespace, name, namespaced := strings.Cut(s, ":")
	if !namespaced {
		namespace, name = "", s
	}
	if namespaced && namespace == "" {
		return ID{}, fmt.Errorf("%w: %q has no framework before its colon", ErrInvalidID, s)
	}
	if name == "" {
		return ID{}, fmt.Errorf("%w: %q names nothing after its framework", ErrInvalidID, s)
	}
	if strings.Contains(name, ":") {
		// No id carries a second colon. The llm brick serves a model under everything
		// after the last one, so an id that did would be served under the wrong name.
		// It is also what tells a compact source key from an id.
		return ID{}, fmt.Errorf("%w: %q holds a second colon, so it reads as a download source", ErrInvalidID, s)
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return ID{}, fmt.Errorf("%w: %q has an empty or traversing path segment", ErrInvalidID, s)
		}
	}
	return ID{namespace: namespace, name: name}, nil
}

// Namespace is the framework the model runs under, "llamacpp" or "genie" or "ei". Empty
// for the bare ids that make up most of models-list.yaml.
func (id ID) Namespace() string { return id.namespace }

// Name is everything after the namespace, slashes included.
func (id ID) Name() string { return id.name }

// Repository is the part of the name that says where the file came from, "unsloth/repo".
// Empty for a name the catalog declares, which is a file stem and nothing else.
func (id ID) Repository() string {
	if i := strings.LastIndex(id.name, "/"); i != -1 {
		return id.name[:i]
	}
	return ""
}

// FileName is the last segment of the name, which is the GGUF file stem. It is the short
// title to show a user; Repository is the context to show beside it.
func (id ID) FileName() string {
	if i := strings.LastIndex(id.name, "/"); i != -1 {
		return id.name[i+1:]
	}
	return id.name
}

func (id ID) String() string {
	if id.namespace == "" {
		return id.name
	}
	return id.namespace + ":" + id.name
}

// PathSegment escapes the id for use as one segment of a request path. A name carrying a
// repository path holds slashes, and a router splits an unescaped one into segments, so
// GET and DELETE answer 404 for an id sent raw.
func (id ID) PathSegment() string { return url.PathEscape(id.String()) }

// MarshalJSON implements the json.Marshaler interface for ID.
func (id ID) MarshalJSON() ([]byte, error) { return json.Marshal(id.String()) }

// Equal implements the go-cmp equality interface.
func (id ID) Equal(other ID) bool {
	return id.namespace == other.namespace && id.name == other.name
}
