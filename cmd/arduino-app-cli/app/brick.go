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

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app/generator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newBrickCmd(cfg config.Configuration) *cobra.Command {
	brickCmd := &cobra.Command{
		Use:   "brick",
		Short: "Manage the bricks of an Arduino App",
	}

	brickCmd.AddCommand(newBrickNewCmd(cfg))
	brickCmd.AddCommand(newBrickAddCmd(cfg))
	brickCmd.AddCommand(newBrickConfigCmd(cfg))

	return brickCmd
}

func newBrickNewCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new app_path name",
		Short: "Create a local brick in an Arduino App",
		Long: `Create a local brick in an Arduino App.

The brick is generated in the bricks folder of the app. Add it to the app with
"app brick add", and edit its brick_compose.yaml for the containers it needs.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			brickNewHandler(args[0], args[1])
			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return completion.ApplicationNames(cfg)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	return cmd
}

func brickNewHandler(appRef, name string) {
	arduinoApp, err := Load(appRef)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}
	// A release is what a build froze: it takes no new brick.
	if _, err := arduinoApp.GetAsEditable(); err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}

	id, err := generator.BrickIDFromName(name)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}
	if err := generator.GenerateLocalBrick(arduinoApp.GetBricksPath(), id, name); err != nil {
		if errors.Is(err, generator.ErrBrickAlreadyExists) {
			feedback.Fatal(fmt.Sprintf("A brick with id %q already exists", id), feedback.ErrBadArgument)
		}
		feedback.Fatal(fmt.Sprintf("Cannot create the brick: %s", err), feedback.ErrGeneric)
	}

	// The path the editor shows, not the absolute one.
	feedback.PrintResult(brickNewResult{Brick: id, Path: paths.New("bricks", id).String()})
}

type brickNewResult struct {
	Brick string `json:"brick"`
	Path  string `json:"path"`
}

func (r brickNewResult) String() string {
	// The brick is ignored until the app lists it.
	return fmt.Sprintf("✓ Created brick %s in %s\n"+
		"  Add it to the app: arduino-app-cli app brick add . %s",
		r.Brick, r.Path, r.Brick)
}

func (r brickNewResult) Data() any {
	return r
}

func newBrickAddCmd(cfg config.Configuration) *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "add app_path brick_id",
		Short: "Add a brick to an Arduino App",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			brickAddHandler(cmd.Context(), args[0], args[1], model)
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

func brickAddHandler(ctx context.Context, appRef, brickID, model string) {
	arduinoApp, err := Load(appRef)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}
	editable, err := arduinoApp.GetAsEditable()
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}

	req := bricks.BrickCreateUpdateRequest{ID: brickID}
	if model != "" {
		req.Model = &model
	}
	if err := servicelocator.GetBrickService().BrickCreate(ctx, req, editable); err != nil {
		feedback.Fatal(fmt.Sprintf("Cannot add the brick: %s", err), feedback.ErrBadArgument)
	}

	feedback.PrintResult(brickAddResult{Brick: brickID, Model: model})
}

type brickAddResult struct {
	Brick string `json:"brick"`
	Model string `json:"model,omitempty"`
}

func (r brickAddResult) String() string {
	if r.Model != "" {
		return fmt.Sprintf("✓ Added brick %s with model %s", r.Brick, r.Model)
	}
	return fmt.Sprintf("✓ Added brick %s", r.Brick)
}

func (r brickAddResult) Data() any {
	return r
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

	// Check that all requested variables exists
	for _, arg := range variableArgs {
		name, _, _ := strings.Cut(arg, "=")
		if name == "" {
			feedback.Fatal(fmt.Sprintf("%q is not a variable name", arg), feedback.ErrBadArgument)
		}
		if _, ok := brick.GetVariable(name); !ok {
			feedback.Fatal(fmt.Sprintf("Variable %q does not exist on brick %q", name, brickID), feedback.ErrBadArgument)
		}
	}

	variables := make(map[string]string, len(variableArgs))
	for _, arg := range variableArgs {
		name, value, hasValue := strings.Cut(arg, "=")
		if !hasValue {
			var err error
			variable, _ := brick.GetVariable(name)
			value, err = feedback.InputUserField(name, variable.Secret)
			if err != nil {
				feedback.Fatal(fmt.Sprintf("cannot read value for %q: %v", name, err), feedback.ErrBadArgument)
			}
		}
		variables[name] = value
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
	return fmt.Sprintf("✓ Updated %s on brick %s", strings.Join(changed, " and "), r.Brick)
}

func (r brickConfigResult) Data() any {
	return r
}
