// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newSearchCmd(cfg config.Configuration) *cobra.Command {
	var allVersions, deprecated bool
	cmd := &cobra.Command{
		Use:     "search [keywords...]",
		Short:   "Search bricks in the bricks index",
		Long:    "Search bricks in the bricks index. Without keywords, all the bricks are listed.",
		Example: "  arduino-app-cli brick search camera usb --all --deprecated",
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			searchHandler(args, allVersions, deprecated, cfg)
		},
	}
	cmd.Flags().BoolVarP(&allVersions, "all", "a", false, "Show the details of all the versions of the bricks")
	cmd.Flags().BoolVar(&deprecated, "deprecated", false, "Include the deprecated bricks")
	return cmd
}

func searchHandler(keywords []string, allVersions, deprecated bool, cfg config.Configuration) {
	index, err := bricksindex.LoadRemoteIndex(cfg.BricksIndexDir())
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(searchResult{
		Bricks:      index.Search(strings.Join(keywords, " "), deprecated),
		allVersions: allVersions,
	})
}

type searchResult struct {
	Bricks      []bricksindex.RemoteBrick `json:"bricks"`
	allVersions bool
}

func (r searchResult) String() string {
	if len(r.Bricks) == 0 {
		return "No bricks matching your search."
	}

	var out strings.Builder
	for _, b := range r.Bricks {
		fmt.Fprintf(&out, "ID: %s\n", b.ID)
		fmt.Fprintf(&out, "  Name: %s\n", b.Name)
		fmt.Fprintf(&out, "  Author: %s\n", b.Namespace)
		fmt.Fprintf(&out, "  Latest: %s\n", b.Latest)
		if b.Deprecated {
			fmt.Fprintf(&out, "  Deprecated: %s\n", cmp.Or(b.DeprecationNote, "yes"))
		}
		if r.allVersions {
			out.WriteString("  Releases:\n")
			for _, release := range b.Releases {
				fmt.Fprintf(&out, "    %s\n", release.Version)
				fmt.Fprintf(&out, "      Description: %s\n", release.Description)
				fmt.Fprintf(&out, "      Repository: %s\n", release.Repository)
			}
			continue
		}
		if latest := b.LatestRelease(); latest != nil {
			fmt.Fprintf(&out, "  Description: %s\n", latest.Description)
			fmt.Fprintf(&out, "  Repository: %s\n", latest.Repository)
		}
		versions := make([]string, 0, len(b.Releases))
		for _, release := range b.Releases {
			versions = append(versions, release.Version)
		}
		fmt.Fprintf(&out, "  Versions: %s\n", strings.Join(versions, ", "))
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func (r searchResult) Data() any {
	return r
}
