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

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
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
		err = brickService.BrickCreate(req, f.Must(app.Load(tempDummyApp.String())))
		require.Nil(t, err)
		after, err := app.Load(tempDummyApp.String())
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 2)
		require.Equal(t, "arduino:dbstorage_sqlstore", after.Descriptor.Bricks[1].ID)
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

		err = brickService.BrickCreate(req, f.Must(app.Load(tempDummyApp.String())))
		require.Nil(t, err)

		after, err := app.Load(tempDummyApp.String())
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 1)
		require.Equal(t, "arduino:arduino_cloud", after.Descriptor.Bricks[0].ID)
		require.Equal(t, deviceID, after.Descriptor.Bricks[0].Variables["ARDUINO_DEVICE_ID"])
		require.Equal(t, secret, after.Descriptor.Bricks[0].Variables["ARDUINO_SECRET"])
	})
}
