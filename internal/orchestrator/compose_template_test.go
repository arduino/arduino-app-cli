// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/stretchr/testify/assert"
)

func TestFrozenLookup(t *testing.T) {
	// The registry is read when the configuration is built, so what the environment says
	// afterwards is what a start would see: the frozen file must keep the build's answer.
	t.Setenv("DOCKER_REGISTRY_BASE", "build.example/")
	cfg := setTestOrchestratorConfig(t)
	t.Setenv("DOCKER_REGISTRY_BASE", "start.example/")
	t.Setenv("LOG_LEVEL", "debug")

	lookup := frozenLookup(cfg, types.Mapping{
		"MODEL_PROMPT": "cost is 5$ a day",
		// A secret is in appEnv as a reference to itself.
		"API_KEY": "${API_KEY}",
	})

	tests := []struct {
		name     string
		variable string
		want     string
		wantSet  bool
	}{
		{
			name:     "a host fact stays the reference the render step answers",
			variable: "APP_HOME",
			want:     "${APP_HOME}",
			wantSet:  true,
		},
		{
			name:     "an app value goes in with its $ escaped",
			variable: "MODEL_PROMPT",
			want:     "cost is 5$$ a day",
			wantSet:  true,
		},
		{
			name:     "a secret stays a reference",
			variable: "API_KEY",
			want:     "${API_KEY}",
			wantSet:  true,
		},
		{
			name:     "the registry is the one the build resolved",
			variable: "DOCKER_REGISTRY_BASE",
			want:     "build.example/",
			wantSet:  true,
		},
		{
			name:     "a variable no brick declares comes from the environment",
			variable: "LOG_LEVEL",
			want:     "debug",
			wantSet:  true,
		},
		{
			name:     "a variable no one declares is not answered",
			variable: "NOT_DECLARED_ANYWHERE",
			want:     "",
			wantSet:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, set := lookup(test.variable)
			assert.Equal(t, test.wantSet, set)
			assert.Equal(t, test.want, value)
		})
	}
}
