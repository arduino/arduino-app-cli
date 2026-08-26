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

// hostFuncs are the only thing a compose template can call: each reads one fact of
// this board, leaving what to write around it to the template shipped with the app.
var hostFuncs = template.FuncMap{
	"groupID":     hostGroupID,
	"deviceMajor": hostDeviceMajor,
	"pathExists":  func(path string) bool { return paths.New(path).Exist() },
}

// renderComposeFile writes the compose file the app is started with: the templates
// evaluated on this board, with their layers and includes merged.
func renderComposeFile(ctx context.Context, arduinoApp *app.ArduinoApp, envs AppEnv) error {
	templateFile := arduinoApp.AppComposeTemplateFilePath()
	content, err := templateFile.ReadFile()
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", templateFile, err)
	}
	rendered, err := renderComposeTemplate(content)
	if err != nil {
		return fmt.Errorf("cannot render %s: %w", templateFile, err)
	}
	configFiles := []types.ConfigFile{{Filename: templateFile.String(), Content: rendered}}

	prj, err := loader.LoadWithContext(ctx,
		types.ConfigDetails{
			ConfigFiles: configFiles,
			// The templates and the composes they include live here.
			WorkingDir: arduinoApp.ProvisioningStateDir().String(),
			// The environment of the process wins, as it does for docker compose.
			Environment: types.NewMapping(os.Environ()).Merge(envs.All()),
		},
		// Relative paths are resolved now: the rendered file is read from elsewhere.
		func(o *loader.Options) { o.ResolvePaths = true },
	)
	if err != nil {
		return err
	}

	// Marshaled by compose-go, which escapes every $ it writes, so reading the
	// file back gives the values resolved here.
	data, err := prj.MarshalYAML()
	if err != nil {
		return err
	}
	composeFile := arduinoApp.AppComposeFilePath()
	if err := fatomic.WriteFile(composeFile.String(), data, 0644); err != nil {
		return err
	}
	slog.Debug("wrote the app compose file", slog.String("path", composeFile.String()))

	return nil
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

// hostGroupID is the id a group has on this board, nothing when it has no such
// group: the same name can have a different id elsewhere.
func hostGroupID(name string) string {
	group, err := user.LookupGroup(name)
	if err != nil {
		slog.Warn("group not found on host; skipping", "group", name)
		return ""
	}
	return group.Gid
}

// hostDeviceMajor is the major number of a char device driver on this board, nothing
// when the driver is not loaded.
func hostDeviceMajor(driver string) string {
	content, err := os.ReadFile("/proc/devices")
	if err != nil {
		slog.Warn("cannot read /proc/devices", slog.Any("error", err))
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == driver {
			return fields[0]
		}
	}
	slog.Warn("driver not found in /proc/devices; skipping", slog.String("driver", driver))
	return ""
}
