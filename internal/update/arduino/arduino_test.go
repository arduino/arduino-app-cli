package arduino

import (
	"fmt"
	"testing"

	semver "go.bug.st/relaxed-semver"
)

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
			name:            "Major update blocked explicit config",
			installed:       "1.9.9",
			available:       []string{"2.0.0", "1.9.10"},
			maxMajorConfig:  1,
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

			name:            "Do not downgrade when installed is above maxAllowedMajor",
			installed:       "2.3.0",
			available:       []string{"1.9.10", "2.4.0"},
			maxMajorConfig:  1,
			expectedVersion: "2.4.0",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var constraint semver.Constraint
			var err error

			if tt.maxMajorConfig == 0 {
				constraint, err = semver.ParseConstraint(">=0.0.0")
			} else {
				constraint, err = semver.ParseConstraint(fmt.Sprintf("<%d.0.0", tt.maxMajorConfig+1))
			}

			if err != nil {
				t.Fatalf("failed to create constraint: %v", err)
			}

			got, err := findBestCandidate(tt.installed, tt.available, constraint)

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
