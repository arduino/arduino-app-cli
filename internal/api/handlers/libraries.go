// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

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
