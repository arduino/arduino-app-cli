// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package models

type ErrorResponse struct {
	Code    string `json:"code,omitempty"`
	Details string `json:"details"`
}
