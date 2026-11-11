// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import "github.com/arduino/arduino-app-cli/internal/appbook"

type (
	ListAppRequest    = appbook.ListAppRequest
	ListAppResult     = appbook.ListAppResult
	AppInfo           = appbook.AppInfo
	AppDetailedInfo   = appbook.AppDetailedInfo
	BrokenAppInfo     = appbook.BrokenAppInfo
	CreateAppRequest  = appbook.CreateAppRequest
	CreateAppResponse = appbook.CreateAppResponse
	CloneAppRequest   = appbook.CloneAppRequest
	CloneAppResponse  = appbook.CloneAppResponse
	AppEditRequest    = appbook.AppEditRequest
	ConfigResponse    = appbook.ConfigResponse
	ExampleResponse   = appbook.ExampleResponse
)

var (
	CreateApp             = appbook.CreateApp
	CloneApp              = appbook.CloneApp
	SetDefaultApp         = appbook.SetDefaultApp
	GetDefaultApp         = appbook.GetDefaultApp
	GetOrchestratorConfig = appbook.GetConfig
	ExportAppZip          = appbook.ExportAppZip
	ImportAppFromZip      = appbook.ImportAppFromZip
)
