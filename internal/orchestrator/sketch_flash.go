// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"slices"
)

type MenuOptions []MenuOption

type MenuOption struct {
	name   string
	values []string
}

var WaitForApp = MenuOptionValue{name: "wait_linux_boot", value: "app"}

type MenuOptionValue struct {
	name  string
	value string
}

func (o MenuOptionValue) String() string {
	return o.name + "=" + o.value
}

func (o MenuOptions) Has(optionValue MenuOptionValue) bool {
	return slices.ContainsFunc(o, func(option MenuOption) bool {
		if option.name == optionValue.name {
			return slices.Contains(option.values, optionValue.value)
		}
		return false
	})
}
