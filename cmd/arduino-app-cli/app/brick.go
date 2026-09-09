// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newBrickCmd(cfg config.Configuration) *cobra.Command {
	brickCmd := &cobra.Command{
		Use:   "brick",
		Short: "Manage the bricks of an Arduino App",
	}

	brickCmd.AddCommand(newBrickConfigCmd(cfg))

	return brickCmd
}

func newBrickConfigCmd(cfg config.Configuration) *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "config app_path brick_id [name=value...]",
		Short: "Configure a brick of an Arduino App",
		Long: `Configure a brick of an Arduino App.

The variables are given as name=value pairs, and an empty value clears one. Only the
variables named are changed, as the app API does it.

An app installed from a release takes one change and no other: its secrets. A build
freezes everything else, and a secret is what a build cannot ship, so the board the
release is installed on is where the values are set. A name that is not a secret of
the brick is refused there, and so is the model.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			variables, err := parseBrickVariables(args[2:])
			if err != nil {
				return err
			}
			if len(variables) == 0 && model == "" {
				return errors.New("give a name=value pair or the model to change")
			}
			brickConfigHandler(cmd.Context(), args[0], args[1], variables, model)
			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return completion.ApplicationNames(cfg)(cmd, args, toComplete)
			case 1:
				return completion.BrickIDs()(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "The AI model the brick runs, by id")

	return cmd
}

func brickConfigHandler(ctx context.Context, appRef, brickID string, variables map[string]string, model string) {
	arduinoApp, err := Load(appRef)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}

	req := bricks.BrickCreateUpdateRequest{ID: brickID, Variables: variables}
	if model != "" {
		req.Model = &model
	}
	err = servicelocator.GetBrickService().BrickUpdate(ctx, req, arduinoApp)
	switch {
	case errors.Is(err, bricks.ErrReleaseSecretsOnly), errors.Is(err, app.ErrReleaseReadOnly):
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	case err != nil:
		feedback.Fatal(fmt.Sprintf("Cannot configure the brick: %s", err), feedback.ErrGeneric)
	}

	names := make([]string, 0, len(variables))
	for name := range variables {
		names = append(names, name)
	}
	slices.Sort(names)
	feedback.PrintResult(brickConfigResult{Brick: brickID, Variables: names, Model: model})
}

// parseBrickVariables reads the name=value pairs. A value is not validated here: what a
// variable may hold is what the brick that reads it accepts.
func parseBrickVariables(args []string) (map[string]string, error) {
	variables := make(map[string]string, len(args))
	for _, arg := range args {
		name, value, found := strings.Cut(arg, "=")
		if !found || name == "" {
			return nil, fmt.Errorf("%q is not a name=value pair", arg)
		}
		variables[name] = value
	}
	return variables, nil
}

// The names are reported and never the values: a variable may be a secret, and a secret
// must not reach a log or a terminal that is scrolled back.
type brickConfigResult struct {
	Brick     string   `json:"brick"`
	Variables []string `json:"variables,omitempty"`
	Model     string   `json:"model,omitempty"`
}

func (r brickConfigResult) String() string {
	var changed []string
	if len(r.Variables) > 0 {
		changed = append(changed, strings.Join(r.Variables, ", "))
	}
	if r.Model != "" {
		changed = append(changed, "model "+r.Model)
	}
	return fmt.Sprintf("✓ Set %s on brick %s", strings.Join(changed, " and "), r.Brick)
}

func (r brickConfigResult) Data() any {
	return r
}
