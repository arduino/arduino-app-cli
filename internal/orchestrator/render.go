// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"strings"
	"text/template"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

// hostFuncs are the only thing a template can call: each reads one fact of the board,
// leaving what to write around it to the template.
var hostFuncs = template.FuncMap{
	"pathExists": func(path string) bool { return paths.New(path).Exist() },

	"groupID": func(name string) string {
		group, err := user.LookupGroup(name)
		if err != nil {
			slog.Warn("group not found on host; skipping", slog.String("group", name))
			return ""
		}
		return group.Gid
	},

	"deviceMajor": func(driver string) string {
		devices, err := os.ReadFile("/proc/devices")
		if err != nil {
			slog.Warn("cannot read /proc/devices", slog.Any("error", err))
			return ""
		}
		for line := range strings.SplitSeq(string(devices), "\n") {
			if fields := strings.Fields(line); len(fields) == 2 && fields[1] == driver {
				return fields[0]
			}
		}
		slog.Warn("driver not loaded on host; skipping", slog.String("driver", driver))
		return ""
	},
}

// renderComposeFile writes the compose file the app is started with: the template
// evaluated on this board, with its includes merged in.
func renderComposeFile(ctx context.Context, arduinoApp *app.ArduinoApp, env, secrets types.Mapping) (*types.Project, error) {
	templateFile := arduinoApp.AppComposeTemplateFilePath()
	content, err := templateFile.ReadFile()
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", templateFile, err)
	}
	rendered, err := renderComposeTemplate(content)
	if err != nil {
		return nil, fmt.Errorf("cannot render %s: %w", templateFile, err)
	}
	configFiles := []types.ConfigFile{{Filename: templateFile.String(), Content: rendered}}

	prj, err := loader.LoadWithContext(ctx,
		types.ConfigDetails{
			ConfigFiles: configFiles,
			// The templates and the composes they include live here.
			WorkingDir: arduinoApp.ProvisioningStateDir().String(),
			// Only what we answer: a template references the host facts and the
			// secrets, so the environment of the cli has nothing to say here.
			Environment: secrets.Clone().Merge(env),
		},
		// Relative paths are resolved now: the rendered file is read from elsewhere.
		func(o *loader.Options) { o.ResolvePaths = true },
	)
	if err != nil {
		return nil, err
	}

	// Marshaled by compose-go, which escapes every $ it writes, so reading the
	// file back gives the values resolved here.
	data, err := prj.MarshalYAML()
	if err != nil {
		return nil, err
	}
	composeFile := arduinoApp.AppComposeFilePath()
	if err := fatomic.WriteFile(composeFile.String(), data, 0644); err != nil {
		return nil, err
	}
	slog.Debug("wrote the app compose file", slog.String("path", composeFile.String()))

	return prj, nil
}

// renderComposeTemplate evaluates the expressions of a compose template: a value
// becomes what it renders to, and a list item that renders to nothing is dropped.
func renderComposeTemplate(content []byte) ([]byte, error) {
	var document any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	document, err := renderComposeNode(document)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(document)
}

func renderComposeNode(node any) (any, error) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			rendered, err := renderComposeNode(child)
			if err != nil {
				return nil, err
			}
			// A key whose value renders to nothing is not in the file.
			if rendered == nil {
				delete(value, key)
				continue
			}
			value[key] = rendered
		}
		return value, nil

	case []any:
		items := make([]any, 0, len(value))
		// Two expressions can resolve to the same thing, and compose rejects a
		// repeated group or rule. Only what was rendered is deduplicated.
		rendered := map[string]bool{}
		for _, child := range value {
			expression, isExpression := child.(string)
			isExpression = isExpression && strings.Contains(expression, "{{")

			item, err := renderComposeNode(child)
			if err != nil {
				return nil, err
			}
			switch item := item.(type) {
			case nil:
			case string:
				if isExpression && rendered[item] {
					continue
				}
				rendered[item] = true
				items = append(items, item)
			default:
				items = append(items, item)
			}
		}
		return items, nil

	case string:
		return renderComposeValue(value)

	default:
		return node, nil
	}
}

func renderComposeValue(value string) (any, error) {
	if !strings.Contains(value, "{{") {
		return value, nil
	}
	// An unknown function is an error here, before anything is started.
	parsed, err := template.New("compose").Funcs(hostFuncs).Parse(value)
	if err != nil {
		return nil, err
	}
	var rendered strings.Builder
	if err := parsed.Execute(&rendered, nil); err != nil {
		return nil, err
	}

	out := strings.TrimSpace(rendered.String())
	if out == "" {
		return nil, nil
	}
	// A value that is not a scalar renders json, which is yaml.
	if strings.HasPrefix(out, "{") {
		var mapping map[string]any
		if err := yaml.Unmarshal([]byte(out), &mapping); err != nil {
			return nil, fmt.Errorf("cannot read the result of %q: %w", value, err)
		}
		return mapping, nil
	}
	return out, nil
}
