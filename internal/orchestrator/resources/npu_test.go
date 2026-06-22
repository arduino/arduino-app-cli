// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNPULine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    NPUSample
		wantErr string
	}{
		{
			name: "typical output",
			line: "NPU0 (CDSP) q6_utilization=39.73 q6_clock=768000 hvx_utilization=10.00 hmx_utilization=5.00",
			want: NPUSample{Q6Utilization: 39.73, Q6Clock: 768000, HVXUtilization: 10.00, HMXUtilization: 5.00},
		},
		{
			name: "zero utilization",
			line: "NPU0 (CDSP) q6_utilization=0.00 q6_clock=0 hvx_utilization=0.00 hmx_utilization=0.00",
			want: NPUSample{},
		},
		{
			name: "full utilization",
			line: "NPU0 (CDSP) q6_utilization=100.00 q6_clock=1000000 hvx_utilization=50.00 hmx_utilization=75.00",
			want: NPUSample{Q6Utilization: 100.00, Q6Clock: 1000000, HVXUtilization: 50.00, HMXUtilization: 75.00},
		},
		{
			name:    "invalid q6_utilization value",
			line:    "NPU0 (CDSP) q6_utilization=bad q6_clock=768000",
			wantErr: `invalid q6_utilization value "bad"`,
		},
		{
			name:    "invalid q6_clock value",
			line:    "NPU0 (CDSP) q6_utilization=10.0 q6_clock=bad",
			wantErr: `invalid q6_clock value "bad"`,
		},
		{
			name:    "no known fields",
			line:    "NPU0 (CDSP) unknown=123",
			wantErr: "no known fields found in line",
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: "no known fields found in line",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseNPULine(tc.line)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tc.want.Q6Utilization, got.Q6Utilization, 0.001)
			assert.Equal(t, tc.want.Q6Clock, got.Q6Clock)
			assert.InDelta(t, tc.want.HVXUtilization, got.HVXUtilization, 0.001)
			assert.InDelta(t, tc.want.HMXUtilization, got.HMXUtilization, 0.001)
		})
	}
}
