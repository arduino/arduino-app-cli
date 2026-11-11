// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"cmp"
	"slices"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

// Search returns, for each brick that contains every word of query ignoring case, its latest release
// that supports the platform. The results are sorted by id. An empty query matches all the
// bricks. Deprecated releases are skipped unless includeDeprecated is set.
func (idx *RemoteIndex) Search(query string, includeDeprecated bool, platform platform.Platform) []*RemoteBrick {
	terms := strings.Fields(strings.ToLower(query))
	res := []*RemoteBrick{}
	seen := map[string]bool{}
	for _, b := range idx.Bricks {
		if seen[b.ID] {
			continue
		}
		seen[b.ID] = true
		latest := idx.Latest(b.ID, platform)
		if latest == nil || (latest.Deprecated && !includeDeprecated) {
			continue
		}
		if latest.matches(terms) {
			res = append(res, latest)
		}
	}
	slices.SortFunc(res, func(a, b *RemoteBrick) int { return cmp.Compare(a.ID, b.ID) })
	return res
}

// matches reports whether every term is contained in the release, ignoring case.
func (b RemoteBrick) matches(terms []string) bool {
	// TODO: should we match the older releases as well, the --all flag is passed here?
	text := strings.ToLower(strings.Join([]string{b.ID, b.Name, b.Namespace, b.Description, b.Repository}, " "))
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}
