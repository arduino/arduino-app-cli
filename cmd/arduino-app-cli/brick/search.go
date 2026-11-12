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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newSearchCmd(cfg config.Configuration, version string) *cobra.Command {
	var deprecated bool
	cmd := &cobra.Command{
		Use:     "search [keywords...]",
		Short:   "Search bricks in the bricks index",
		Long:    "Search bricks in the bricks index. Without keywords, all the bricks are listed.",
		Example: "  arduino-app-cli brick search camera usb --deprecated",
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			searchHandler(cmd.Context(), args, deprecated, cfg, version)
		},
	}
	cmd.Flags().BoolVar(&deprecated, "deprecated", false, "Include the deprecated bricks")
	return cmd
}

func searchHandler(ctx context.Context, keywords []string, deprecated bool, cfg config.Configuration, version string) {
	index := loadRemoteIndex(ctx, cfg, version)
	platform := servicelocator.GetPlatform()
	res := searchResult{Bricks: []searchItem{}}
	for _, brick := range index.Search(strings.Join(keywords, " "), deprecated, platform) {
		item := searchItem{
			ID:              brick.ID,
			Namespace:       brick.Namespace,
			Author:          brick.Author,
			Name:            brick.Name,
			Description:     brick.Description,
			Deprecated:      brick.Deprecated,
			DeprecationNote: brick.DeprecationNote,
			Latest:          brick.Latest(platform).Version.String(),
		}
		for _, release := range brick.SupportedReleases(platform) {
			item.Versions = append(item.Versions, release.Version.String())
		}
		res.Bricks = append(res.Bricks, item)
	}
	feedback.PrintResult(res)
}

// searchItem is a brick, with the versions of its releases that support the board.
type searchItem struct {
	ID              string   `json:"id"`
	Namespace       string   `json:"namespace"`
	Author          string   `json:"author"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Deprecated      bool     `json:"deprecated"`
	DeprecationNote string   `json:"deprecation_note,omitempty"`
	Latest          string   `json:"latest"`
	Versions        []string `json:"versions"`
}

type searchResult struct {
	Bricks []searchItem `json:"bricks"`
}

func (r searchResult) String() string {
	if len(r.Bricks) == 0 {
		return "No bricks matching your search."
	}

	var out strings.Builder
	for _, b := range r.Bricks {
		fmt.Fprintf(&out, "ID: %s\n", b.ID)
		fmt.Fprintf(&out, "  Name: %s\n", b.Name)
		fmt.Fprintf(&out, "  Author: %s\n", b.Author)
		fmt.Fprintf(&out, "  Latest: %s\n", b.Latest)
		if b.Deprecated {
			fmt.Fprintf(&out, "  Deprecated: %s\n", cmp.Or(b.DeprecationNote, "yes"))
		}
		fmt.Fprintf(&out, "  Description: %s\n", b.Description)
		fmt.Fprintf(&out, "  Versions: %s\n", strings.Join(b.Versions, ", "))
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func (r searchResult) Data() any {
	return r
}
