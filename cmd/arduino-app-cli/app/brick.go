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
		Use:   "config app_path brick_id name[=value] [name[=value] ...]",
		Short: "Set the value of a brick's variables in an Arduino App",
		Long: `Set the value of a brick's variables in an Arduino App.

Variables given as "name" without a value will be prompted for a value on the terminal.
Variables given as "name=value" will be changed, without any prompt.
Variables given as "name=" with an empty value will be cleared, without any prompt.

On an App Release, only secret variables can be changed.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			brickConfigHandler(cmd.Context(), args[0], args[1], args[2:], model)
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

func brickConfigHandler(ctx context.Context, appRef, brickID string, variableArgs []string, model string) {
	arduinoApp, err := Load(appRef)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}

	brick, ok := arduinoApp.Bricks(servicelocator.GetBricksIndex()).FindBrickByID(brickID)
	if !ok {
		feedback.Fatal(fmt.Sprintf("Cannot find the brick with ID %q", brickID), feedback.ErrBadArgument)
	}

	inputValue := func(name string) (string, error) {
		variable, ok := brick.GetVariable(name)
		if !ok {
			feedback.Fatal(fmt.Sprintf("Variable %q does not exist on brick %q", name, brickID), feedback.ErrBadArgument)
		}
		return feedback.InputUserField(name, variable.Secret)
	}

	variables, err := parseBrickVariables(variableArgs, inputValue)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}
	if len(variables) == 0 && model == "" {
		feedback.Fatal("give a variable or the model to change", feedback.ErrBadArgument)
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

// parseBrickVariables reads the variable arguments. A value is not validated here: what
// a variable may hold is what the brick that reads it accepts.
func parseBrickVariables(args []string, input func(name string) (string, error)) (map[string]string, error) {
	variables := make(map[string]string, len(args))
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		if name == "" {
			return nil, fmt.Errorf("%q is not a variable name", arg)
		}
		if !hasValue {
			var err error
			value, err = input(name)
			if err != nil {
				return nil, fmt.Errorf("cannot read value for %q: %w", name, err)
			}
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
