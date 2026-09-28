// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package helpers

// LastPercent drops what would report the same percentage twice: a download is
// reported by the chunk, and read by the percent.
type LastPercent int

func (last *LastPercent) Moved(curr, total int64) bool {
	if total <= 0 {
		return false
	}
	percent := LastPercent(curr * 100 / total)
	if percent == *last {
		return false
	}
	*last = percent
	return true
}
