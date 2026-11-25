// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

// newProgress shows the messages of an app operation. Call done before the result or an error is printed.
func newProgress() (progress func(orchestrator.StreamMessage), done func()) {
	p := feedback.NewProgress()
	return func(message orchestrator.StreamMessage) {
		if progress := message.GetProgress(); progress != nil {
			p.Update(progress.Name, progress.Progress)
		}
		if message.IsData() && message.IsTool() {
			p.Detail(message.GetData())
		} else if message.IsData() {
			p.Println(message.GetData())
		}
	}, p.Done
}
