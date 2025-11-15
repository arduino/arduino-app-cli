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
	"errors"
	"slices"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func TestBrickCreate(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(nil, bricksIndex, nil)

	t.Run("fails if brick id does not exist", func(t *testing.T) {
		err = brickService.BrickCreate(BrickCreateUpdateRequest{ID: "not-existing-id"}, f.Must(app.Load("testdata/dummy-app")))
		require.Error(t, err)
		require.Equal(t, "brick \"not-existing-id\" not found", err.Error())
	})

	t.Run("fails if the requestes variable is not present in the brick definition", func(t *testing.T) {
		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"NON_EXISTING_VARIABLE": "some-value",
		}}
		err = brickService.BrickCreate(req, f.Must(app.Load("testdata/dummy-app")))
		require.Error(t, err)
		require.Equal(t, "variable \"NON_EXISTING_VARIABLE\" does not exist on brick \"arduino:arduino_cloud\"", err.Error())
	})

	t.Run("fails if a required variable is set empty", func(t *testing.T) {
		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"ARDUINO_DEVICE_ID": "",
			"ARDUINO_SECRET":    "a-secret-a",
		}}
		err = brickService.BrickCreate(req, f.Must(app.Load("testdata/dummy-app")))
		require.Error(t, err)
		require.Equal(t, "variable \"ARDUINO_DEVICE_ID\" cannot be empty", err.Error())
	})

	t.Run("fails if a mandatory variable is not present in the request", func(t *testing.T) {
		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"ARDUINO_SECRET": "a-secret-a",
		}}
		err = brickService.BrickCreate(req, f.Must(app.Load("testdata/dummy-app")))
		require.Error(t, err)
		require.Equal(t, "required variable \"ARDUINO_DEVICE_ID\" is mandatory", err.Error())
	})

	t.Run("the brick is added if it does not exist in the app", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app.temp")
		err := tempDummyApp.RemoveAll()
		require.Nil(t, err)
		require.Nil(t, paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp))

		req := BrickCreateUpdateRequest{ID: "arduino:dbstorage_sqlstore"}
		before := f.Must(app.Load(tempDummyApp.String()))
		err = brickService.BrickCreate(req, before)
		require.Nil(t, err)
		after, err := app.Load(tempDummyApp.String())
		require.Nil(t, err)
		requireBricksSizeUpdatedBy(t, before.Descriptor, after.Descriptor, 1)
		requireBricksContain(t, after.Descriptor, "arduino:dbstorage_sqlstore")
	})

	t.Run("the variables of a brick are updated", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app.brick-override.temp")
		err := tempDummyApp.RemoveAll()
		require.Nil(t, err)
		err = paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp)
		require.Nil(t, err)
		bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata"))
		require.Nil(t, err)
		brickService := NewService(nil, bricksIndex, nil)

		deviceID := "this-is-a-device-id"
		secret := "this-is-a-secret"
		req := BrickCreateUpdateRequest{
			ID: "arduino:arduino_cloud",
			Variables: map[string]string{
				"ARDUINO_DEVICE_ID": deviceID,
				"ARDUINO_SECRET":    secret,
			},
		}

		before := f.Must(app.Load(tempDummyApp.String()))
		err = brickService.BrickCreate(req, before)
		require.Nil(t, err)

		after, err := app.Load(tempDummyApp.String())
		require.Nil(t, err)
		requireBricksSizeUpdatedBy(t, before.Descriptor, after.Descriptor, 0)
		requireBricksContain(t, after.Descriptor, "arduino:arduino_cloud")
		require.Equal(t, deviceID, after.Descriptor.Bricks[0].Variables["ARDUINO_DEVICE_ID"])
		require.Equal(t, secret, after.Descriptor.Bricks[0].Variables["ARDUINO_SECRET"])
	})
}

func TestGetBrickInstanceVariableDetails(t *testing.T) {
	tests := []struct {
		name                    string
		brick                   *bricksindex.Brick
		userVariables           map[string]string
		expectedConfigVariables []BrickConfigVariable
		expectedVariableMap     map[string]string
	}{
		{
			name: "variable is present in the map",
			brick: &bricksindex.Brick{
				Variables: []bricksindex.BrickVariable{
					{Name: "VAR1", Description: "desc"},
				},
			},
			userVariables: map[string]string{"VAR1": "value1"},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VAR1", Value: "value1", Description: "desc", Required: true},
			},
			expectedVariableMap: map[string]string{"VAR1": "value1"},
		},
		{
			name: "variable not present in the map",
			brick: &bricksindex.Brick{
				Variables: []bricksindex.BrickVariable{
					{Name: "VAR1", Description: "desc"},
				},
			},
			userVariables: map[string]string{},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VAR1", Value: "", Description: "desc", Required: true},
			},
			expectedVariableMap: map[string]string{"VAR1": ""},
		},
		{
			name: "variable with default value",
			brick: &bricksindex.Brick{
				Variables: []bricksindex.BrickVariable{
					{Name: "VAR1", DefaultValue: "default", Description: "desc"},
				},
			},
			userVariables: map[string]string{},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VAR1", Value: "default", Description: "desc", Required: false},
			},
			expectedVariableMap: map[string]string{"VAR1": "default"},
		},
		{
			name: "multiple variables",
			brick: &bricksindex.Brick{
				Variables: []bricksindex.BrickVariable{
					{Name: "VAR1", Description: "desc1"},
					{Name: "VAR2", DefaultValue: "def2", Description: "desc2"},
				},
			},
			userVariables: map[string]string{"VAR1": "v1"},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VAR1", Value: "v1", Description: "desc1", Required: true},
				{Name: "VAR2", Value: "def2", Description: "desc2", Required: false},
			},
			expectedVariableMap: map[string]string{"VAR1": "v1", "VAR2": "def2"},
		},
		{
			name:                    "no variables",
			brick:                   &bricksindex.Brick{Variables: []bricksindex.BrickVariable{}},
			userVariables:           map[string]string{},
			expectedConfigVariables: []BrickConfigVariable{},
			expectedVariableMap:     map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualVariableMap, actualConfigVariables := getBrickConfigDetails(tt.brick, tt.userVariables)
			require.Equal(t, tt.expectedVariableMap, actualVariableMap)
			require.Equal(t, tt.expectedConfigVariables, actualConfigVariables)
		})
	}
}

func TestBrickCreateWithModulesUpdate(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata"))
	require.Nil(t, err)
	modelsIndex, err := modelsindex.GenerateModelsIndexFromFile(paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(modelsIndex, bricksIndex, nil)
	model := "glass-breaking"
	missingModel := "missing-model"

	tests := []struct {
		name                         string
		model                        string
		updateRequest                BrickCreateUpdateRequest
		expectedErr                  error
		expectedBrickModelDescriptor string
	}{
		{
			name:  "add a brick with a defined model to an App",
			model: "glass-breaking",
			updateRequest: BrickCreateUpdateRequest{
				ID:        "arduino:audio_classification",
				Variables: nil,
				Model:     &model,
			},
			expectedErr:                  nil,
			expectedBrickModelDescriptor: "arduino:audio_classification",
		},
		{
			name:  "add a brick with not existent model to an App",
			model: "glass-breaking",
			updateRequest: BrickCreateUpdateRequest{
				ID:        "arduino:audio_classification",
				Variables: nil,
				Model:     &missingModel,
			},
			expectedErr:                  errors.New("model missing-model does not exist"),
			expectedBrickModelDescriptor: "arduino:audio_classification",
		},
	}

	for _, tt := range tests {
		dummyApp := appDummySetup(t)
		t.Run(tt.name, func(t *testing.T) {

			brickErr := brickService.BrickCreate(tt.updateRequest, f.Must(app.Load(dummyApp.String())))
			require.Equal(t, brickErr, tt.expectedErr)

			after, err := app.Load(dummyApp.String())
			require.Nil(t, err)
			brickAddedToApp := slices.ContainsFunc(after.Descriptor.Bricks, func(b app.Brick) bool {
				return b.ID == tt.expectedBrickModelDescriptor
			})
			if brickErr != nil {
				require.False(t, brickAddedToApp)
			} else {
				require.True(t, brickAddedToApp)
			}

		})
	}
}

func TestAppBrickInstanceDetails(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata"))
	require.Nil(t, err)
	modelsIndex, err := modelsindex.GenerateModelsIndexFromFile(paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(modelsIndex, bricksIndex, nil)

	tests := []struct {
		name                 string
		brickId              string
		expectedErrorMessage string
		expectedModelId      string
	}{
		{
			name:                 "details for a brick defined in the app",
			brickId:              "arduino:video_object_detection",
			expectedErrorMessage: "",
			expectedModelId:      "yolox-object-detection",
		},

		{
			name:                 "details should be not available for a brick not defined in the app",
			brickId:              "arduino:audio_classification",
			expectedErrorMessage: "brick arduino:audio_classification not added in the app",
		},

		{
			name:                 "details for a not exitent brick",
			brickId:              "arduino:notExistentBrick",
			expectedErrorMessage: "brick not found",
		},
	}

	for _, tt := range tests {
		dummyApp := appDummySetup(t)
		ymlApp := f.Must(app.Load(dummyApp.String()))
		brickInstance, err := brickService.AppBrickInstanceDetails(&ymlApp, tt.brickId)
		if err == nil {
			require.Equal(t, tt.brickId, brickInstance.ID)
			require.Equal(t, tt.expectedModelId, brickInstance.ModelID)
		} else {
			require.Equal(t, tt.expectedErrorMessage, err.Error())
		}
	}
}

func appDummySetup(t *testing.T) *paths.Path {
	tempDummyApp := paths.New("testdata/dummy-app.temp")
	err := tempDummyApp.RemoveAll()
	require.Nil(t, err)
	require.Nil(t, paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp))
	return tempDummyApp
}

func requireBricksSizeUpdatedBy(t *testing.T, before app.AppDescriptor, after app.AppDescriptor, value int) {
	require.Len(t, after.Bricks, len(before.Bricks)+value)
}

// getBrickIndexByBrickId searches the Bricks slice within the AppDescriptor
// for a Brick whose ID matches the provided brickId.
func getBrickIndexByBrickId(application app.AppDescriptor, brickId string) int {
	idx := slices.IndexFunc(application.Bricks, func(b app.Brick) bool {
		return brickId == b.ID
	})
	return idx
}

func requireBricksContain(t *testing.T, application app.AppDescriptor, brickID string) {
	idx := getBrickIndexByBrickId(application, brickID)
	require.NotEqual(t, idx, -1)
}
