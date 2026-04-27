package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerProgressParser_Parse(t *testing.T) {

	testCases := []struct {
		name             string
		historySize      int
		logLines         []string
		expectedProgress []uint64
		finalState       map[string]*layerProgress
	}{
		{
			name:        "Single layer download should report increasing progress",
			historySize: 3,
			logLines: []string{
				"  e756f3fdd6a3 Downloading [===>                     ]  1.5MB/10.0MB", // 1.5/10 = 15% history: [15.0] (15.0) / historySize = 15.0 / 1 = 15
				"  e756f3fdd6a3 Downloading [=========>               ]  5.0MB/10.0MB", // 5.0/10 = 50% history: [15.0, 50.0] (15.0 + 50.0) / historySize = 65.0 / 2 = 32.5
				"  irrelevant log",
				"  e756f3fdd6a3 Downloading [===================>     ]  9.0MB/10.0MB",  // 9.0/10 = 90% history: [15.0, 50, 90.0] (15.0 + 50.0 + 90.0) / historySize = 155.0.0 / 3 = 51
				"  e756f3fdd6a3 Downloading [=====================>   ]  10.0MB/10.0MB", // 10/10 = 100% history: [50, 90.0, 100.0] (50.0 + 90.0 +100.0) / historySize = 240.0 / 3 = 80
			},
			expectedProgress: []uint64{15, 32, 51, 80},
		},
		{
			name:        "Two parallel downloads should aggregate progress correctly",
			historySize: 2,
			logLines: []string{
				"  e756f3fdd6a3 Downloading [====> ]  5.0MB/10.0MB",
				"  a555d2abb4b2 Downloading [>     ]  2.0MB/40.0MB",
				"  e756f3fdd6a3 Downloading [=====>]  10.0MB/10.0MB",
				"  a555d2abb4b2 Downloading [=====>]  20.0MB/40.0MB",
				"  a555d2abb4b2 Downloading [=====>]  40.0MB/40.0MB",
			}, // just 2 results instead of 5 because we avoit to returs equal values: if [1] = 50% and [2] = 50%, we retun nil second time
			expectedProgress: []uint64{50, 80},
		},
		{
			name:        "Irrelevant lines should be ignored",
			historySize: 5,
			logLines: []string{
				"  main Pulling",
				"  e756f3fdd6a3 Pulling fs layer",
				"  e756f3fdd6a3 Waiting",
				"  e756f3fdd6a3 Verifying Checksum",
				"  e756f3fdd6a3 Download complete",
				"  e756f3fdd6a3 Extracting [>   ] 1.0MB/10.0MB", // only one good line
			},
			expectedProgress: []uint64{10},
		},
		{
			name:        "Parser should handle different byte units not showing the last",
			historySize: 1,
			logLines: []string{
				"  e756f3fdd6a3 Downloading [> ]  512.0kB/2.0MB", // 25%
				"  e756f3fdd6a3 Downloading [> ]  1.5MB/2.0MB",   // 75%
				"  a555d2abb4b2 Downloading [> ]  1.0GB/2.0GB",   // Raw: (1.5MB+1GB)/(2MB+2GB) ~= 50%
			}, // the last one, 50%, is lower than the previous one 75%. So we don't return it and we keep 75%
			expectedProgress: []uint64{25, 75},
		},
		{
			name:        "Parser should handle different byte units showing the last",
			historySize: 1,
			logLines: []string{
				"  e756f3fdd6a3 Downloading [> ]  512.0kB/2.0MB", // 25%
				"  e756f3fdd6a3 Downloading [> ]  1.5MB/2.0MB",   // 75%
				"  a555d2abb4b2 Downloading [> ]  2.0GB/2.0GB",   // 100%
			},
			expectedProgress: []uint64{25, 75, 99},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewDockerProgressParser(tc.historySize)

			var reportedProgress []uint64

			for _, line := range tc.logLines {
				if progress, ok := parser.Parse(line); ok {
					reportedProgress = append(reportedProgress, progress)
				}
			}

			assert.Equal(t, tc.expectedProgress, reportedProgress, "The reported progress sequence should match the expected one")
		})
	}
}
