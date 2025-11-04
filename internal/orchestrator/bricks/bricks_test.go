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
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/go-paths-helper"
)

func TestOverrideBrickVariablesOfApp(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(nil, bricksIndex, nil)

	deviceID := fmt.Sprintf("my-device-id-%x", rand.Int())
	secret := fmt.Sprintf("my-device-secret-%x", rand.Int())

	req := BrickCreateUpdateRequest{
		ID: "arduino:arduino_cloud",
		Variables: map[string]string{
			"ARDUINO_DEVICE_ID": deviceID,
			"ARDUINO_SECRET":    secret,
		},
	}

	err = brickService.BrickCreate(req, f.Must(app.Load("./testdata/my-app")))
	require.Nil(t, err)

	after, err := app.Load("./testdata/my-app")
	require.Nil(t, err)
	require.Len(t, after.Descriptor.Bricks, 1)
	require.Equal(t, "arduino:arduino_cloud", after.Descriptor.Bricks[0].ID)
	require.Equal(t, deviceID, after.Descriptor.Bricks[0].Variables["ARDUINO_DEVICE_ID"])
	require.Equal(t, secret, after.Descriptor.Bricks[0].Variables["ARDUINO_SECRET"])
}
