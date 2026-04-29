// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !linux
// +build !linux

package micro

import "fmt"

func enableOnBoard(string, int) error {
	return fmt.Errorf("micro is not supported on this platform")
}

func disableOnBoard(string, int) error {
	return fmt.Errorf("Enable is not supported on this platform")
}
