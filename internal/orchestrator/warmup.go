// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"sync"

	composeschema "github.com/compose-spec/compose-go/v2/schema"
	"github.com/gosimple/slug"
)

var prewarmOnce sync.Once

// Prewarm pays, in the background, what the render of an app otherwise pays the first
// time it runs in a process: the compilation of the compose schema and the tables of
// the slug of a project name. On the board they are ~175 ms and ~100 ms, which a start
// overlaps with the loading of the indexes instead of waiting for on its critical path.
func Prewarm() {
	prewarmOnce.Do(func() {
		go func() { _ = composeschema.Validate(map[string]any{}) }()
		go func() { _ = slug.Make("prewarm") }()
	})
}
