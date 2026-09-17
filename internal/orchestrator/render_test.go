// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// render is what the resolve step would write, rendered and read back.
func render(t *testing.T, template string) map[string]any {
	t.Helper()

	rendered, err := renderComposeTemplate([]byte(template))
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, yaml.Unmarshal(rendered, &document))
	return document
}

func TestRenderComposeTemplate(t *testing.T) {
	existing := t.TempDir()

	t.Run("a value becomes what it renders to", func(t *testing.T) {
		document := render(t, `
services:
  main:
    image: busybox
    working_dir: '`+exprPrefix+`{{ if pathExists "`+existing+`" }}/here{{ end }}'
`)
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.Equal(t, "/here", main["working_dir"])
		require.Equal(t, "busybox", main["image"], "a value without an expression is untouched")
	})

	t.Run("a key that renders to nothing is dropped", func(t *testing.T) {
		document := render(t, `
services:
  main:
    working_dir: '`+exprPrefix+`{{ if pathExists "`+existing+`/missing" }}/here{{ end }}'
`)
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.NotContains(t, main, "working_dir")
	})

	t.Run("a list item that renders to nothing is dropped", func(t *testing.T) {
		document := render(t, `
services:
  main:
    group_add:
      - '`+exprPrefix+`{{ if pathExists "`+existing+`" }}44{{ end }}'
      - '`+exprPrefix+`{{ if pathExists "`+existing+`/missing" }}29{{ end }}'
      - "1000"
`)
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.Equal(t, []any{"44", "1000"}, main["group_add"])
	})

	t.Run("two expressions resolving to the same value are deduplicated", func(t *testing.T) {
		document := render(t, `
services:
  main:
    device_cgroup_rules:
      - '`+exprPrefix+`{{ if pathExists "`+existing+`" }}c 226:* rmw{{ end }}'
      - '`+exprPrefix+`{{ if pathExists "`+existing+`" }}c 226:* rmw{{ end }}'
`)
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.Equal(t, []any{"c 226:* rmw"}, main["device_cgroup_rules"])
	})

	t.Run("a literal repeated by hand is kept", func(t *testing.T) {
		document := render(t, `
services:
  main:
    command: ["sh", "-c", "x", "x"]
`)
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.Equal(t, []any{"sh", "-c", "x", "x"}, main["command"])
	})

	t.Run("a mount renders as a mapping", func(t *testing.T) {
		expr, err := mountExpr(existing + ":ro")
		require.NoError(t, err)

		document := render(t, "services:\n  main:\n    volumes:\n      - '"+expr+"'\n")
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.Equal(t, []any{map[string]any{
			"type":      "bind",
			"source":    existing,
			"target":    existing,
			"read_only": true,
			"bind":      map[string]any{"create_host_path": false},
		}}, main["volumes"])
	})

	t.Run("a mount the board has not is dropped", func(t *testing.T) {
		expr, err := mountExpr(existing + "/missing")
		require.NoError(t, err)

		document := render(t, "services:\n  main:\n    volumes:\n      - '"+expr+"'\n")
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.Empty(t, main["volumes"])
	})

	t.Run("a value the app brings along is left alone", func(t *testing.T) {
		document := render(t, `
name: '{{ my-app }}'
services:
  main:
    environment:
      PROMPT: "Hello {{ name }}, reply in {{ .Language }}"
      TOPIC: "{{ if x }}a{{ end }}"
    working_dir: '`+exprPrefix+`{{ if pathExists "`+existing+`" }}/here{{ end }}'
`)
		main := document["services"].(map[string]any)["main"].(map[string]any)
		env := main["environment"].(map[string]any)
		require.Equal(t, "Hello {{ name }}, reply in {{ .Language }}", env["PROMPT"],
			"a brick variable holding a template of its own is not ours to evaluate")
		require.Equal(t, "{{ if x }}a{{ end }}", env["TOPIC"])
		require.Equal(t, "{{ my-app }}", document["name"], "nor is anything else the app names")
		require.Equal(t, "/here", main["working_dir"], "what the resolve step marked still renders")
	})

	t.Run("an unknown function is an error", func(t *testing.T) {
		_, err := renderComposeTemplate([]byte("services:\n  main:\n    image: '" + exprPrefix + "{{ notAFunction \"x\" }}'\n"))
		require.ErrorContains(t, err, `function "notAFunction" not defined`)
	})
}

func TestServicesOverrides(t *testing.T) {
	appEnv := types.Mapping{"FOO": "bar"}
	user := "1000:1000"
	withUser := "root"

	overrides := servicesOverrides([]serviceInfo{
		{name: "plain"},
		{name: "with-devices", requireDevices: true},
		{name: "with-user", user: &withUser},
	}, user, appEnv, []string{"drm"}, []string{"video"})

	require.Len(t, overrides, 3)

	data, err := yaml.Marshal(map[string]any{"services": overrides})
	require.NoError(t, err)
	var document struct {
		Services map[string]map[string]any `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(data, &document))

	for name, override := range document.Services {
		require.Equal(t, "true", override["labels"].(map[string]any)[DockerAppLabel], name)
		require.Equal(t, "bar", override["environment"].(map[string]any)["FOO"], name)
		require.Equal(t, []any{exprPrefix + `{{ groupID "video" }}`}, override["group_add"], name)
	}

	require.Equal(t, user, document.Services["plain"]["user"], "the user is set when the service declares none")
	require.NotContains(t, document.Services["with-user"], "user", "a service declaring a user keeps it")

	require.NotContains(t, document.Services["plain"], "device_cgroup_rules")
	require.Equal(t, []any{exprPrefix + `{{ with deviceMajor "drm" }}c {{ . }}:* rmw{{ end }}`},
		document.Services["with-devices"]["device_cgroup_rules"])
	require.NotEmpty(t, document.Services["with-devices"]["volumes"], "/dev is mounted")
}

// TestRenderComposeFileWithIncludedBrick renders an app whose brick ships a compose: its
// services are included by the main template and overridden by the second one.
func TestRenderComposeFileWithIncludedBrick(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)

	arduinoApp := app.ArduinoApp{
		Name: "TestApp",
		Descriptor: app.AppDescriptor{
			Bricks: []app.Brick{{ID: "arduino:video_object_detection"}},
		},
		FullPath: paths.New(t.TempDir()),
	}
	require.NoError(t, arduinoApp.ProvisioningStateDir().MkdirAll())

	brickPath := cfg.AssetDir().Join("compose", "arduino", "video_object_detection")
	require.NoError(t, brickPath.MkdirAll())
	require.NoError(t, brickPath.Join("brick_compose.yaml").WriteFile([]byte(`
services:
  ei-video-obj-detection-runner:
    image: arduino/video-object-detection:latest
`)))
	require.NoError(t, cfg.AssetDir().Join("bricks-list.yaml").WriteFile([]byte(`
bricks:
- id: arduino:video_object_detection
  name: Object Detection
  category: video
`)))
	require.NoError(t, cfg.AssetDir().Join("services").MkdirAll())
	servicesIndex, err := servicesindex.Load(platform.GetPlatform(nil), cfg.AssetDir().Join("services"))
	require.NoError(t, err)
	bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), cfg.AssetDir())
	require.NoError(t, err)

	// A brick is free to declare a variable holding a template of its own.
	appEnv := types.Mapping{"FOO": "bar", "PROMPT": "Hello {{ name }}, reply"}
	require.NoError(t, generateComposeTemplate(&arduinoApp, arduinoApp.ProvisioningStateDir(), bricksIndex,
		servicesIndex, "python-apps-base:latest", cfg, appEnv, unkownPlatform))

	env := hostEnvironment(t.Context(), arduinoApp.FullPath, cfg).Merge(appEnv)
	prj, err := renderComposeFile(t.Context(), &arduinoApp, env, types.Mapping{}, "test-app")
	require.NoError(t, err)
	require.True(t, arduinoApp.AppComposeFilePath().Exist(), "the compose file docker is given should exist")

	runner, err := prj.GetService("ei-video-obj-detection-runner")
	require.NoError(t, err, "the service the brick declares should be included")
	require.Equal(t, "arduino/video-object-detection:latest", runner.Image, "what the brick declares is kept")
	require.Equal(t, "true", runner.Labels[DockerAppLabel], "the override is applied over it")
	require.Equal(t, "bar", *runner.Environment["FOO"])
	require.Equal(t, "Hello {{ name }}, reply", *runner.Environment["PROMPT"], "a value of the app is passed through")
	require.Equal(t, arduinoApp.FullPath.String(), *runner.Environment["APP_HOME"], "a host fact is answered")

	main, err := prj.GetService("main")
	require.NoError(t, err)
	require.Equal(t, "python-apps-base:latest", main.Image)
	require.Equal(t, "Hello {{ name }}, reply", *main.Environment["PROMPT"])
	require.Contains(t, main.DependsOn, "ei-video-obj-detection-runner")
}
