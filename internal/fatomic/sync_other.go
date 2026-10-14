// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !linux

package fatomic

// Sync does nothing: the cli runs on Linux, other systems only build it for dev.
func Sync() {}
