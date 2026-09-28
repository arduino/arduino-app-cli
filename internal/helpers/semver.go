// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package helpers

import (
	"slices"

	semver "go.bug.st/relaxed-semver"
)

// SelectBestVersion returns the highest version in available that satisfies the
// given constraint. Versions older than installed are discarded: a downgrade is
// never selected. It returns nil when no version qualifies.
func SelectBestVersion(available []string, installed *semver.Version, constraint semver.Constraint) *semver.Version {
	candidates := make([]*semver.Version, 0, len(available))

	for _, verStr := range available {
		v, err := semver.Parse(verStr)
		if err != nil {
			continue
		}

		if !constraint.Match(v) {
			continue
		}
		if installed != nil && v.LessThan(installed) {
			continue
		}

		candidates = append(candidates, v)
	}

	if len(candidates) == 0 {
		return nil
	}

	slices.SortFunc(candidates, func(a, b *semver.Version) int {
		return a.CompareTo(b)
	})

	return candidates[len(candidates)-1]
}
