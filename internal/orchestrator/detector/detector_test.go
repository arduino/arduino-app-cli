package detector

import (
	"testing"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectLocalAppBricks(t *testing.T) {

	t.Run("it detects local bricks in an app", func(t *testing.T) {
		bricks, err := detectLocalAppBricks(paths.New("testdata/AppWithLocalBricks"))
		require.NoError(t, err)
		assert.Len(t, bricks, 2)

		want := []AppLocalBrick{
			{
				FullPath:    paths.New("testdata/AppWithLocalBricks/my_first_brick"),
				ConfigFile:  paths.New("testdata/AppWithLocalBricks/my_first_brick/brick_config.yaml"),
				ComposeFile: nil,
				Brick: bricksindex.Brick{
					ID:          "dneri:my_first_brick",
					Name:        "My First Brick",
					Description: "This is my first brick",
				},
			},
			{
				FullPath:    paths.New("testdata/AppWithLocalBricks/my_second_brick"),
				ConfigFile:  paths.New("testdata/AppWithLocalBricks/my_second_brick/brick_config.yaml"),
				ComposeFile: paths.New("testdata/AppWithLocalBricks/my_second_brick/brick_compose.yaml"),
				Brick: bricksindex.Brick{
					ID:          "dneri:my_second_brick",
					Name:        "My Second Brick",
					Description: "This is my second brick",
				},
			},
		}
		assert.Equal(t, want, bricks)
	})
}
