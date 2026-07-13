// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"reflect"
	"testing"
)

func TestImagesToPull(t *testing.T) {
	tests := []struct {
		name          string
		images        []string
		alreadyPulled []string
		want          []string
	}{
		{
			name:          "none present pulls all in order",
			images:        []string{"a:1", "b:2", "c:3"},
			alreadyPulled: nil,
			want:          []string{"a:1", "b:2", "c:3"},
		},
		{
			name:          "some present are skipped, order preserved",
			images:        []string{"a:1", "b:2", "c:3"},
			alreadyPulled: []string{"b:2"},
			want:          []string{"a:1", "c:3"},
		},
		{
			name:          "all present pulls nothing",
			images:        []string{"a:1", "b:2"},
			alreadyPulled: []string{"b:2", "a:1", "z:9"},
			want:          nil,
		},
		{
			name:          "no images",
			images:        nil,
			alreadyPulled: []string{"a:1"},
			want:          nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imagesToPull(tt.images, tt.alreadyPulled)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("imagesToPull(%v, %v) = %v, want %v", tt.images, tt.alreadyPulled, got, tt.want)
			}
		})
	}
}
