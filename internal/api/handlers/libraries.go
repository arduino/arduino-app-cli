// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// HandleLibraryList is a proxy to the List libraries API
func HandleLibraryList(target *url.URL, version string) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL = target
			r.Out.URL.RawQuery = r.In.URL.RawQuery
			r.Out.Host = target.Host // Cloudfront needs the Host header to match the URL host otherwise it returns 403
			r.Out.Header.Set("User-Agent", "arduino-app-cli/"+version)

			r.SetXForwarded()
			slog.Debug("Proxying library request", slog.Any("in", r.In.URL), slog.Any("out", r.Out.URL), slog.String("target", target.String()))
		},
	}
}

// NOTE: this is only to generate the openapi docs.
type LibraryListResponse struct {
	Libraries  []Library  `json:"libraries"`
	Pagination Pagination `json:"pagination"`
}

type Library struct {
	Name string `json:"name"`
	ID   string `json:"id"`

	Repository *struct {
		URL       string `json:"url"`
		Stars     int    `json:"stars"`
		Forks     int    `json:"forks"`
		UpdatedAt string `json:"updated_at"`
	} `json:"repository"`
	Website string `json:"website"`
	License string `json:"license"`

	Platform      *string  `json:"platform"`
	Architectures []string `json:"architectures"`
	Types         []string `json:"types"`
	Category      string   `json:"category"`

	Maintainer string `json:"maintainer"`
	Author     string `json:"author"`
	Sentence   string `json:"sentence"`
	Paragraph  string `json:"paragraph"`

	Includes     []string `json:"includes"`
	Dependencies []struct {
		Name string `json:"name"`
	} `json:"dependencies"`

	ExampleCount int `json:"example_count"`

	Releases []struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	} `json:"releases"`
}

type Pagination struct {
	TotalPages int `json:"total_pages"`
	TotalItems int `json:"total_items"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	NextPage   int `json:"next_page"`
	PrevPage   int `json:"prev_page"`
}
