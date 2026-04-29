// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package fatomic

import (
	"os"

	"github.com/google/renameio/v2"
)

func WriteFile(filename string, data []byte, perm os.FileMode, opts ...renameio.Option) error {
	return renameio.WriteFile(filename, data, perm)
}
