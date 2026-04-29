// This file is part of arduino-app-cli.
//
// Copyright 2020 ARDUINO SA (http://www.arduino.cc/)
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package i18n

import "fmt"

type Locale interface {
	Get(msg string, args ...any) string
}

type nullLocale struct{}

func (n nullLocale) Parse([]byte) {}

func (n nullLocale) Get(msg string, args ...any) string {
	return fmt.Sprintf(msg, args...)
}

var locale Locale = &nullLocale{}

func SetLocale(l Locale) {
	locale = l
}

// Tr returns msg translated to the selected locale
// the msg argument must be a literal string
func Tr(msg string, args ...any) string {
	return locale.Get(msg, args...)
}
