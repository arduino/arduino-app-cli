package arduino

import "testing"

func TestFindBestCandidate(t *testing.T) {
	tests := []struct {
		name            string
		installed       string
		available       []string
		maxMajorConfig  int
		expectedVersion string
		expectError     bool
	}{
		{
			name:            "Standard update: minor upgrade available",
			installed:       "1.0.0",
			available:       []string{"1.0.1", "1.1.0"},
			maxMajorConfig:  0,
			expectedVersion: "1.1.0",
			expectError:     false,
		},
		{
			name:            "Major update blocked by default (Config=0)",
			installed:       "1.9.9",
			available:       []string{"2.0.0", "1.9.10"},
			maxMajorConfig:  0,
			expectedVersion: "1.9.10",
			expectError:     false,
		},
		{
			name:            "Major update allowed by explicit config",
			installed:       "1.9.9",
			available:       []string{"2.0.0", "3.0.0"},
			maxMajorConfig:  2,
			expectedVersion: "2.0.0",
			expectError:     false,
		},
		{
			name:            "CRITICAL: Regression test for 'Zero Value' bug (Version 2+)",
			installed:       "2.1.0",
			available:       []string{"2.2.0", "3.0.0"},
			maxMajorConfig:  0,
			expectedVersion: "2.2.0",
			expectError:     false,
		},
		{
			name:            "No updates available (all older or same)",
			installed:       "1.5.0",
			available:       []string{"1.0.0", "1.5.0"},
			maxMajorConfig:  0,
			expectedVersion: "",
			expectError:     false,
		},
		{
			name:            "Handle unsorted list and pick highest valid",
			installed:       "1.0.0",
			available:       []string{"1.1.0", "1.5.0", "1.2.0"},
			maxMajorConfig:  0,
			expectedVersion: "1.5.0",
			expectError:     false,
		},
		{
			name:            "Skip invalid candidate strings",
			installed:       "1.0.0",
			available:       []string{"invalid-ver", "1.1.0"},
			maxMajorConfig:  0,
			expectedVersion: "1.1.0",
			expectError:     false,
		},
		{
			name:            "Error on invalid installed version string",
			installed:       "not-a-semver",
			available:       []string{"1.0.0"},
			maxMajorConfig:  0,
			expectedVersion: "",
			expectError:     true,
		},
		{
			name:            "Prerelease handling (standard logic ignores prereleases unless specifically handled)",
			installed:       "1.0.0",
			available:       []string{"1.0.1-beta"},
			maxMajorConfig:  0,
			expectedVersion: "1.0.1-beta",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findBestCandidate(tt.installed, tt.available, tt.maxMajorConfig)

			if (err != nil) != tt.expectError {
				t.Errorf("findBestCandidate() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if got != tt.expectedVersion {
				t.Errorf("findBestCandidate() = %v, want %v", got, tt.expectedVersion)
			}
		})
	}
}
