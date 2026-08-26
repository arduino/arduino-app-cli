// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
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
    working_dir: '{{ if pathExists "`+existing+`" }}/here{{ end }}'
`)
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.Equal(t, "/here", main["working_dir"])
		require.Equal(t, "busybox", main["image"], "a value without an expression is untouched")
	})

	t.Run("a key that renders to nothing is dropped", func(t *testing.T) {
		document := render(t, `
services:
  main:
    working_dir: '{{ if pathExists "`+existing+`/missing" }}/here{{ end }}'
`)
		main := document["services"].(map[string]any)["main"].(map[string]any)
		require.NotContains(t, main, "working_dir")
	})

	t.Run("a list item that renders to nothing is dropped", func(t *testing.T) {
		document := render(t, `
services:
  main:
    group_add:
      - '{{ if pathExists "`+existing+`" }}44{{ end }}'
      - '{{ if pathExists "`+existing+`/missing" }}29{{ end }}'
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
      - '{{ if pathExists "`+existing+`" }}c 226:* rmw{{ end }}'
      - '{{ if pathExists "`+existing+`" }}c 226:* rmw{{ end }}'
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

	t.Run("an unknown function is an error", func(t *testing.T) {
		_, err := renderComposeTemplate([]byte("services:\n  main:\n    image: '{{ notAFunction \"x\" }}'\n"))
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
		require.Equal(t, []any{`{{ groupID "video" }}`}, override["group_add"], name)
	}

	require.Equal(t, user, document.Services["plain"]["user"], "the user is set when the service declares none")
	require.NotContains(t, document.Services["with-user"], "user", "a service declaring a user keeps it")

	require.NotContains(t, document.Services["plain"], "device_cgroup_rules")
	require.Equal(t, []any{`{{ with deviceMajor "drm" }}c {{ . }}:* rmw{{ end }}`},
		document.Services["with-devices"]["device_cgroup_rules"])
	require.NotEmpty(t, document.Services["with-devices"]["volumes"], "/dev is mounted")
}
