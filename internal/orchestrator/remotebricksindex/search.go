// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

import (
	"strings"
)

// LatestRelease returns the release named by Latest, or nil if there is none.
func (b Brick) LatestRelease() *Release {
	for i := range b.Releases {
		if b.Releases[i].Version == b.Latest {
			return &b.Releases[i]
		}
	}
	return nil
}

// Search returns the bricks that contain every word of query, ignoring case.
// An empty query matches all the bricks. Deprecated bricks are skipped unless includeDeprecated is set.
func (idx *Index) Search(query string, includeDeprecated bool) []Brick {
	terms := strings.Fields(strings.ToLower(query))
	res := []Brick{}
	for _, b := range idx.Bricks {
		if b.Deprecated && !includeDeprecated {
			continue
		}
		if b.matches(terms) {
			res = append(res, b)
		}
	}
	return res
}

// matches reports whether every term is contained in the brick, ignoring case.
func (b Brick) matches(terms []string) bool {
	fields := []string{b.ID, b.Name, b.Namespace}
	// TODO: should we match the older releases as well, the --all flag is passed here?
	if latest := b.LatestRelease(); latest != nil {
		fields = append(fields, latest.Description, latest.Repository)
	}
	text := strings.ToLower(strings.Join(fields, " "))
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}
