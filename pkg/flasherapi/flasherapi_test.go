package flasherapi

import "testing"

func TestParseOSImageVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		found    bool
	}{
		{
			name:     "valid build id",
			input:    "BUILD_ID=\"20251006-395\"\nVARIANT_ID=xfce",
			expected: "20251006-395",
			found:    true,
		},
		{
			name:  "missing build id",
			input: "VARIANT_ID=xfce\n",
			found: false,
		},
		{
			name:  "empty build id",
			input: "BUILD_ID=\n",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseOSImageVersion(tt.input)
			if ok != tt.found || got != tt.expected {
				t.Fatalf("got (%q, %v), expected (%q, %v)", got, ok, tt.expected, tt.found)
			}
		})
	}
}
