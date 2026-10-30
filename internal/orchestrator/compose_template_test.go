// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("a host fact stays as written, default included", func(t *testing.T) {
		frozen, err := frozenYAML([]byte("volumes:\n  - ${HOST_DSP_INSTALLATION_PATH:-/usr/share/qcom}:/usr/share/qcom\n"), lookup)
		require.NoError(t, err)
		assert.Contains(t, string(frozen), "${HOST_DSP_INSTALLATION_PATH:-/usr/share/qcom}:/usr/share/qcom")
	})
}

func TestFrozenComposeIncludes(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	compose := []byte("services:\n  svc:\n    image: busybox\n")

	assetCompose := cfg.AssetDir().Join("compose", "arduino", "asset_brick", "brick_compose.yaml")
	require.NoError(t, assetCompose.Parent().MkdirAll())
	require.NoError(t, assetCompose.WriteFile(compose))

	// A brick the app brings along is outside the asset dir, at a different depth.
	appDir := paths.New(t.TempDir()).Join("home", "arduino", "ArduinoApps", "my-app")
	localCompose := appDir.Join("bricks", "local_brick", "brick_compose.yaml")
	require.NoError(t, localCompose.Parent().MkdirAll())
	require.NoError(t, localCompose.WriteFile(compose))

	genPath := appDir.Join(".cache")
	require.NoError(t, genPath.MkdirAll())

	includes, err := frozenComposeIncludes(paths.NewPathList(assetCompose.String(), localCompose.String()), genPath, cfg, types.Mapping{})
	require.NoError(t, err)
	assert.Equal(t, []composeInclude{
		{Path: "compose/compose/arduino/asset_brick/brick_compose.yaml"},
		{Path: "compose/local_brick/brick_compose.yaml", ProjectDirectory: "${APP_HOME}/bricks/local_brick"},
	}, includes)
	for _, include := range includes {
		assert.FileExists(t, genPath.Join(include.Path).String())
	}

	// A release generates in its prebuild dir, installed as the .cache of the app.
	prebuildDir := paths.New(t.TempDir()).Join("my-app-release", "prebuild")
	require.NoError(t, prebuildDir.MkdirAll())
	includes, err = frozenComposeIncludes(paths.NewPathList(localCompose.String()), prebuildDir, cfg, types.Mapping{})
	require.NoError(t, err)
	assert.Equal(t, []composeInclude{
		{Path: "compose/local_brick/brick_compose.yaml", ProjectDirectory: "${APP_HOME}/bricks/local_brick"},
	}, includes)
}
