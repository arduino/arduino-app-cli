// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	appmodel "github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newDetailsCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:               "details app_path",
		Short:             "Show an Arduino App's details",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ApplicationNames(cfg),
		RunE: func(cmd *cobra.Command, args []string) error {
			arduinoApp, err := Load(args[0])
			if err != nil {
				return err
			}
			return detailsHandler(cmd.Context(), cfg, arduinoApp)
		},
	}
}

func detailsHandler(ctx context.Context, cfg config.Configuration, arduinoApp appmodel.ArduinoApp) error {
	details, err := orchestrator.AppDetails(
		ctx,
		servicelocator.GetDockerClient(),
		arduinoApp,
		servicelocator.GetBricksIndex(),
		servicelocator.GetAppIDProvider(),
		cfg,
	)
	if err != nil {
		return err
	}

	feedback.PrintResult(newAppDetailsResult(details, arduinoApp, servicelocator.GetBricksIndex()))
	return nil
}

type appDetailsResult struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Path        string              `json:"path"`
	Description string              `json:"description"`
	Icon        string              `json:"icon"`
	Status      orchestrator.Status `json:"status"`
	Example     bool                `json:"example"`
	Default     bool                `json:"default"`
	Bricks      []appDetailsBrick   `json:"bricks"`
}

type appDetailsBrick struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Category  string               `json:"category,omitempty"`
	Model     string               `json:"model,omitempty"`
	Variables []appDetailsVariable `json:"variables"`
}

type appDetailsVariable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Configured  bool   `json:"configured"`
}

func newAppDetailsResult(
	details orchestrator.AppDetailedInfo,
	arduinoApp appmodel.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
) appDetailsResult {
	result := appDetailsResult{
		ID:          details.ID.String(),
		Name:        details.Name,
		Path:        details.Path,
		Description: details.Description,
		Icon:        details.Icon,
		Status:      details.Status,
		Example:     details.Example,
		Default:     details.Default,
		Bricks:      make([]appDetailsBrick, len(arduinoApp.Descriptor.Bricks)),
	}

	index := arduinoApp.Bricks(bricksIndex)
	for i, appBrick := range arduinoApp.Descriptor.Bricks {
		brick := appDetailsBrick{ID: appBrick.ID, Model: appBrick.Model}
		if definition, found := index.FindBrickByID(appBrick.ID); found {
			brick.Name = definition.Name
			brick.Category = definition.Category
			for _, variable := range definition.Variables {
				if variable.Hidden {
					continue
				}
				_, configured := appBrick.Variables[variable.Name]
				brick.Variables = append(brick.Variables, appDetailsVariable{
					Name:        variable.Name,
					Description: variable.Description,
					Required:    variable.IsRequired(),
					Configured:  configured,
				})
			}
		}
		result.Bricks[i] = brick
	}
	return result
}

func (r appDetailsResult) String() string {
	var out strings.Builder
	fmt.Fprintf(&out, "ID: %s\nName: %s\n", r.ID, r.Name)
	if r.Description != "" {
		fmt.Fprintf(&out, "Description: %s\n", r.Description)
	}
	if r.Icon != "" {
		fmt.Fprintf(&out, "Icon: %s\n", r.Icon)
	}
	fmt.Fprintf(&out, "Path: %s\nStatus: %s\n", r.Path, r.Status)
	fmt.Fprintf(&out, "Example: %t\nDefault: %t\n", r.Example, r.Default)

	out.WriteString("\nBricks:\n")
	if len(r.Bricks) == 0 {
		out.WriteString("  None\n")
		return strings.TrimSuffix(out.String(), "\n")
	}
	for _, brick := range r.Bricks {
		name := brick.ID
		if brick.Name != "" {
			name = fmt.Sprintf("%s (%s)", brick.ID, brick.Name)
		}
		fmt.Fprintf(&out, "  %s\n", name)
		if brick.Category != "" {
			fmt.Fprintf(&out, "    Category: %s\n", brick.Category)
		}
		if brick.Model != "" {
			fmt.Fprintf(&out, "    Model: %s\n", brick.Model)
		}
		if len(brick.Variables) == 0 {
			out.WriteString("    Variables: none\n")
			continue
		}
		out.WriteString("    Variables:\n")
		for _, variable := range brick.Variables {
			requirement := "optional"
			if variable.Required {
				requirement = "required"
			}
			configured := "not configured"
			if variable.Configured {
				configured = "configured"
			}
			fmt.Fprintf(&out, "      %s (%s, %s)", variable.Name, requirement, configured)
			if variable.Description != "" {
				fmt.Fprintf(&out, ": %s", variable.Description)
			}
			out.WriteByte('\n')
		}
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func (r appDetailsResult) Data() any {
	return r
}
