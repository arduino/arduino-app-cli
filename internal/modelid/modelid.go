// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package modelid encodes the id of an AI model for a URL path.
package modelid

import (
	"encoding/base64"
	"fmt"
)

// Encode renders a model id as one URL path segment: base64url, unpadded, the
// encoding app ids already use. Every id survives it, including the bare ones.
func Encode(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

// Decode reads back what Encode wrote, so an id is plain text below the
// handlers. An id that is not base64url is refused rather than passed through.
func Decode(encoded string) (string, error) {
	id, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: model id must be base64url encoded, unpadded", err)
	}
	return string(id), nil
}
