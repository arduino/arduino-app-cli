// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"io/fs"
	"net/http"
)

func DocsServer(contentFS fs.FS) http.Handler {
	subFS, err := fs.Sub(contentFS, "docs")
	if err != nil {
		panic("embedded docs folder not found" + err.Error())
	}
	return http.FileServer(http.FS(subFS))
}
