// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package helpers

type EnvVars map[string]string

func (e EnvVars) AsList() []string {
	list := make([]string, 0, len(e))
	for k, v := range e {
		list = append(list, k+"="+v)
	}
	return list
}
