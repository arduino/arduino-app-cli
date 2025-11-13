// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package bricks

import (
	"testing"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestBrickCreateWithModulesUpdate(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata"))
	require.Nil(t, err)
	modelsIndex, err := modelsindex.GenerateModelsIndexFromFile(paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(modelsIndex, bricksIndex, nil)

	/*
		t.Run("add a new brick with a model to an App", func(t *testing.T) {
		// we want to verify the model key in the app.yaml
		}
		t.Run("add a new brick with a model and a variable to an App", func(t *testing.T) {
		// we want to verify the model key in the app.yaml
		// we want to verify the model-variable key and value in the app.yaml
		}
		t.Run("add a new brick with a model and a not defined variable to an App", func(t *testing.T) {
		// we want to raise error because the provided variable is not defined in the brick.conf
		// app.yaml fails
		// OR
		// we want to ignore unknown used defined variables (bricks variable behavior)
		}
		t.Run("add a new brick with a not defined model", func(t *testing.T) {
		// we want to raise error because the provided model is not defined in the brick.conf
		// app.yaml fails
		}
		t.Run("add a new brick with a model where we define only a part of the variables decalred in the brick", func(t *testing.T) {
		// we want to raise error because some variables are unset (this means all are required variables)
		// app.yaml fails
		}
		// TODO: could this break calls from upper layers?
		// Update BrickCreateUpdateRequest by adding Model Variables
		req := BrickCreateUpdateRequest{
			ID:        "arduino:test_brick",
			Variables: map[string]string{"TEST_VAR_1": "user-provided-test1"},
			Model:     &model,
			MoldelVariables: map[string]string{"EI_AUDIO_CLASSIFICATION_MODEL": "custom model variable"},
		}
	*/
	t.Run("add a brick to an App", func(t *testing.T) {
		dummyApp := appDummySetup(t)

		model := "glass-breaking"
		req := BrickCreateUpdateRequest{
			ID:        "arduino:test_brick",
			Variables: map[string]string{"BRICK_VAR1": "user-provided-brick-level-var1"},
			Model:     &model,
		}

		// act
		err = brickService.BrickCreate(req, f.Must(app.Load(dummyApp.String())))
		require.Nil(t, err)

		// assert
		after, err := app.Load(dummyApp.String())
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 2)
		require.Equal(t, "arduino:test_brick", after.Descriptor.Bricks[1].ID)
	})

}

func appDummySetup(t *testing.T) *paths.Path {
	tempDummyApp := paths.New("testdata/dummy-app.temp")
	err := tempDummyApp.RemoveAll()
	require.Nil(t, err)
	require.Nil(t, paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp))
	return tempDummyApp
}
