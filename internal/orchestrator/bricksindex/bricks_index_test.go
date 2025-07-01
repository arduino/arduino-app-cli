package bricksindex

import (
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

func TestBricksIndex(t *testing.T) {
	x := `arduino:
  app-bricks:
    package: app-bricks
    repository: github.com/arduino/app-bricks
    latest-release: 2.0.0
    releases:
      - version: 1.0.0
        bricks:
          - id: arduino:dbstorage
            variables:
              PORT:
                default_value: 8080
      - version: 2.0.0
        bricks:
          - id: arduino:dbstorage
            variables:
              PORT:
                default_value: 8080
          - id: arduino:redis
            variables:
              PORT:
                default_value: 8080
                description: port
`

	var b *BricksIndex
	err := yaml.Unmarshal([]byte(x), &b)
	require.NoError(t, err)
}
