// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package linuxconfig

import "go.bug.st/f"

type Carrier struct {
	CarrierName    string   `json:"carrier_name"`
	CurrentEnabled bool     `json:"current_enabled"`
	Devices        []Device `json:"current"`
}

type CarrierStatusOutput struct {
	Carriers []Carrier `json:"carriers"`
}

type Device struct {
	Device     string `json:"device"`      // "camera0", "camera1", "display"
	Option     string `json:"option"`      // "type1-4lanes", "none", etc.
	DeviceType string `json:"device_type"` // "camera", "display"
}

func (c Carrier) EnabledDevices() []Device {
	return f.Filter(c.Devices, func(device Device) bool {
		return device.Option != "none"
	})
}
