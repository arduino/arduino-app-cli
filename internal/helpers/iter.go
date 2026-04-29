// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package helpers

import (
	"iter"
)

func EmptyIter[V any]() iter.Seq[V] {
	return func(yield func(V) bool) {}
}
