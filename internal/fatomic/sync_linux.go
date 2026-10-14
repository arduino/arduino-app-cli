// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package fatomic

import "syscall"

// Sync flushes to disk the pending writes of every filesystem.
func Sync() {
	syscall.Sync()
}
