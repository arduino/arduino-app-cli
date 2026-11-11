// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newSearchCmd(cfg config.Configuration, version string) *cobra.Command {
	var allVersions, deprecated bool
	cmd := &cobra.Command{
		Use:     "search [keywords...]",
		Short:   "Search bricks in the bricks index",
		Long:    "Search bricks in the bricks index. Without keywords, all the bricks are listed.",
		Example: "  arduino-app-cli brick search camera usb --all --deprecated",
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			searchHandler(cmd.Context(), args, allVersions, deprecated, cfg, version)
		},
	}
	cmd.Flags().BoolVarP(&allVersions, "all", "a", false, "Show the details of all the versions of the bricks")
	cmd.Flags().BoolVar(&deprecated, "deprecated", false, "Include the deprecated bricks")
	return cmd
}

func searchHandler(ctx context.Context, keywords []string, allVersions, deprecated bool, cfg config.Configuration, version string) {
	index := loadRemoteIndex(ctx, cfg, version)
	platform := servicelocator.GetPlatform()
	res := searchResult{Bricks: []searchItem{}, allVersions: allVersions}
	for _, latest := range index.Search(strings.Join(keywords, " "), deprecated, platform) {
		item := searchItem{RemoteBrick: latest}
		for _, release := range index.Releases(latest.ID, platform) {
			item.Versions = append(item.Versions, release.Version.String())
			if allVersions {
				item.Releases = append(item.Releases, release)
			}
		}
		res.Bricks = append(res.Bricks, item)
	}
	feedback.PrintResult(res)
}

// searchItem is the latest release of a brick, with the versions of its releases that support the board.
type searchItem struct {
	*bricksindex.RemoteBrick
	Versions []string                   `json:"versions"`
	Releases []*bricksindex.RemoteBrick `json:"releases,omitempty"`
}

type searchResult struct {
	Bricks      []searchItem `json:"bricks"`
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
		fmt.Fprintf(&out, "  Latest: %s\n", b.Version)
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
		fmt.Fprintf(&out, "  Description: %s\n", b.Description)
		fmt.Fprintf(&out, "  Repository: %s\n", b.Repository)
		fmt.Fprintf(&out, "  Versions: %s\n", strings.Join(b.Versions, ", "))
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func (r searchResult) Data() any {
	return r
}
