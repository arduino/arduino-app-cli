// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"strings"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

var _ feedback.Result = (*orchestrator.InitResult)(nil)

// newInitEventCallback writes what `system init` reports: each event as a result without a
// progress, else the source as the step of the progress, which fills its own range.
func newInitEventCallback(printEvent func(feedback.Result), progress *feedback.Progress) orchestrator.InitEventCallback {
	type step struct {
		name     string
		from, to float32
	}
	steps := map[orchestrator.InitEventSource]step{
		orchestrator.InitSourceDeb:     {"Installing the board package", 0, 30},
		orchestrator.InitSourceArduino: {"Downloading platforms and libraries", 30, 60},
		orchestrator.InitSourceDocker:  {"Pulling container images", 60, 100},
	}
	var source orchestrator.InitEventSource
	var percent float32
	var label string
	return func(event orchestrator.InitEvent) {
		if progress == nil {
			if result := orchestrator.NewInitResult(event); result != nil {
				printEvent(result)
			}
			return
		}
		current := steps[event.Source]
		if event.Source != source {
			source, percent = event.Source, current.from
		}
		if event.Type == orchestrator.InitProgressEvent && event.Progress.Total > 0 {
			percent = current.from + (current.to-current.from)*float32(event.Progress.Curr)/float32(event.Progress.Total)
		}
		progress.Update(current.name, percent)
		switch {
		case event.Type == orchestrator.InitLogEvent:
			if event.Source == orchestrator.InitSourceDeb && !aptAlert(event.Message) {
				progress.Detail(event.Message)
				return
			}
			progress.Println(event.Message)
		// What a download is about is printed once, its bytes only move the bar; a docker
		// pull has its own line already.
		case event.Type == orchestrator.InitProgressEvent && event.Source != orchestrator.InitSourceDocker && event.Progress.Label != label:
			label = event.Progress.Label
			progress.Println(label)
		}
	}
}

// aptAlert reports whether an apt line is an error or a warning, printed in the normal
// color; the rest of the apt output is gray.
func aptAlert(line string) bool {
	return strings.HasPrefix(line, "E:") || strings.HasPrefix(line, "W:")
}
