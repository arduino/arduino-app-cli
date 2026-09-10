// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

var _ feedback.Result = (*orchestrator.InitResult)(nil)

// newInitEventCallback writes what `system init` reports. Nothing is dropped here:
// what reports a progress states it once per percent.
func newInitEventCallback(printEvent func(feedback.Result)) orchestrator.InitEventCallback {
	return func(event orchestrator.InitEvent) {
		if result := orchestrator.NewInitResult(event); result != nil {
			printEvent(result)
		}
	}
}
