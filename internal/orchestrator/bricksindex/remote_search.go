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

// Search returns the bricks that contain every word of query ignoring case and have a release that
// supports the platform. The results are sorted by id. An empty query matches all the bricks.
// Deprecated bricks are skipped unless includeDeprecated is set.
func (idx *RemoteIndex) Search(query string, includeDeprecated bool, platform platform.Platform) []*RemoteBrick {
	terms := strings.Fields(strings.ToLower(query))
	res := []*RemoteBrick{}
	for i := range idx.Bricks {
		b := &idx.Bricks[i]
		if (b.Deprecated && !includeDeprecated) || b.Latest(platform) == nil {
			continue
		}
		if b.matches(terms) {
			res = append(res, b)
		}
	}
	slices.SortFunc(res, func(a, b *RemoteBrick) int { return cmp.Compare(a.ID, b.ID) })
	return res
}

// matches reports whether every term is contained in the brick, ignoring case.
func (b RemoteBrick) matches(terms []string) bool {
	text := strings.ToLower(strings.Join([]string{b.ID, b.Name, b.Namespace, b.Author, b.Description}, " "))
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}
