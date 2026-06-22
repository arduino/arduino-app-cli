// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

type ModelInstallEventType string

const (
	ModelInstallEventStart    ModelInstallEventType = "start"
	ModelInstallEventUpdate   ModelInstallEventType = "update"
	ModelInstallEventComplete ModelInstallEventType = "complete"
	ModelInstallEventInfo     ModelInstallEventType = "info"
	ModelInstallEventError    ModelInstallEventType = "error"
	ModelInstallEventDone     ModelInstallEventType = "done"
)

type ModelInstallEvent struct {
	ModelID     string                `json:"model_id"`
	Type        ModelInstallEventType `json:"type"`
	Description string                `json:"description,omitempty"`
	Current     int64                 `json:"current,omitempty"`
	Total       int64                 `json:"total,omitempty"`
	Unit        string                `json:"unit,omitempty"`
	Percentage  string                `json:"percentage,omitempty"`
	Artifacts   []string              `json:"artifacts,omitempty"`
}
