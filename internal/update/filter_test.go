package update

import (
	"testing"

	"github.com/stretchr/testify/require"
	semver "go.bug.st/relaxed-semver"
)

func TestFilterPackagesByMatchers(t *testing.T) {
	matchArduino := func(p UpgradablePackage) bool {
		return p.Type == Arduino
	}

	matchAll := func(p UpgradablePackage) bool {
		return true
	}

	matchDeb := func(p UpgradablePackage) bool {
		return p.Type == Debian
	}

	matchConstraint := func(p UpgradablePackage) bool {
		constraint, err := semver.ParseConstraint("<1.0.0")
		require.NoError(t, err)
		v, err := semver.Parse(p.ToVersion)
		require.NoError(t, err)
		if constraint.Match(v) {
			return true
		}
		return false
	}

	tests := []struct {
		name     string
		given    []UpgradablePackage
		matchers []MatcherFunc
		want     []UpgradablePackage
	}{
		{
			name: "Match all packages",
			given: []UpgradablePackage{
				{Type: Debian, Name: "arduino-cli", FromVersion: "1.0.0", ToVersion: "1.1.0"},
				{Type: Debian, Name: "nano", FromVersion: "2.9.3", ToVersion: "2.9.4"},
				{Type: Debian, Name: "adbd", FromVersion: "1.0.0", ToVersion: "2.0.1"},
				{Type: Arduino, Name: "arduino-cli", FromVersion: "0.52.0", ToVersion: "0.53.0"},
			},
			matchers: []MatcherFunc{matchAll},
			want: []UpgradablePackage{
				{Type: Debian, Name: "arduino-cli", FromVersion: "1.0.0", ToVersion: "1.1.0"},
				{Type: Debian, Name: "nano", FromVersion: "2.9.3", ToVersion: "2.9.4"},
				{Type: Debian, Name: "adbd", FromVersion: "1.0.0", ToVersion: "2.0.1"},
				{Type: Arduino, Name: "arduino-cli", FromVersion: "0.52.0", ToVersion: "0.53.0"},
			},
		},
		{
			name: "Match only arduino packages",
			given: []UpgradablePackage{
				{Type: Debian, Name: "arduino-cli", FromVersion: "1.0.0", ToVersion: "1.1.0"},
				{Type: Debian, Name: "nano", FromVersion: "2.9.3", ToVersion: "2.9.4"},
				{Type: Debian, Name: "adbd", FromVersion: "1.0.0", ToVersion: "2.0.1"},
				{Type: Arduino, Name: "arduino-cli", FromVersion: "0.52.0", ToVersion: "0.53.0"},
			},
			matchers: []MatcherFunc{matchArduino},
			want: []UpgradablePackage{
				{Type: Arduino, Name: "arduino-cli", FromVersion: "0.52.0", ToVersion: "0.53.0"},
			},
		},
		{
			name: "Match only deb packages",
			given: []UpgradablePackage{
				{Type: Debian, Name: "arduino-cli", FromVersion: "1.0.0", ToVersion: "1.1.0"},
				{Type: Debian, Name: "nano", FromVersion: "2.9.3", ToVersion: "2.9.4"},
				{Type: Debian, Name: "adbd", FromVersion: "1.0.0", ToVersion: "2.0.1"},
				{Type: Arduino, Name: "arduino-core", FromVersion: "0.52.0", ToVersion: "0.53.0"},
			},
			matchers: []MatcherFunc{matchDeb},
			want: []UpgradablePackage{
				{Type: Debian, Name: "arduino-cli", FromVersion: "1.0.0", ToVersion: "1.1.0"},
				{Type: Debian, Name: "nano", FromVersion: "2.9.3", ToVersion: "2.9.4"},
				{Type: Debian, Name: "adbd", FromVersion: "1.0.0", ToVersion: "2.0.1"},
			},
		},
		{
			name: "Match deb with semver constraint",
			given: []UpgradablePackage{
				{Type: Debian, Name: "arduino-cli", FromVersion: "1.0.0", ToVersion: "1.1.0"},
				{Type: Debian, Name: "zephyr", FromVersion: "0.52.0", ToVersion: "0.53.0"},
				{Type: Debian, Name: "zephyr", FromVersion: "0.60.0", ToVersion: "1.0.0"},
				{Type: Arduino, Name: "arduino-core", FromVersion: "0.52.0", ToVersion: "0.53.0"},
			},
			matchers: []MatcherFunc{matchDeb, matchConstraint},
			want: []UpgradablePackage{
				{Type: Debian, Name: "zephyr", FromVersion: "0.52.0", ToVersion: "0.53.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterPackagesByMatchers(tt.want, tt.matchers...)
			if len(got) != len(tt.want) {
				t.Errorf("expected %d packages, got %d", len(tt.want), len(got))
			}
			require.Equal(t, tt.want, got)
		})
	}
}
