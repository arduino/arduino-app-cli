// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package linuxconfig

type Carrier struct {
	CarrierName    string `json:"carrier_name"`
	CurrentEnabled bool   `json:"current_enabled"`
}

type CarrierStatusOutput struct {
	Carriers []Carrier `json:"carriers"`
}
