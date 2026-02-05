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

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestBuildBrickConfigForEIModel(t *testing.T) {

	brickIndex, err := bricksindex.Load(paths.New("bricksindex/testdata"))
	if err != nil {
		t.Fatalf("failed to load bricks index: %v", err)
	}

	category := edgeimpulse.ProjectCategory("Object detection")
	edgeModelsDir := paths.New("/models/custom-ei/ei-xxxx-yyyy")
	blobModelsDir := paths.New("/models/custom-ei/ei-xxxx-yyyy")

	result := buildBrickConfigForEIModel(
		brickIndex,
		&category,
		edgeModelsDir,
		blobModelsDir,
	)

	require.Len(t, result, 2)

	require.Equal(t, "arduino:object_detection", result[0].ID)
	require.Equal(t, "arduino:video_object_detection", result[1].ID)

	require.Equal(t, map[string]interface{}{
		"CUSTOM_MODEL_PATH":      "/models/custom-ei/ei-xxxx-yyyy",
		"EI_OBJ_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
	}, result[0].ModelConfiguration)
	require.Equal(t, map[string]interface{}{
		"CUSTOM_MODEL_PATH":      "/models/custom-ei/ei-xxxx-yyyy",
		"EI_OBJ_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
	}, result[1].ModelConfiguration)
}
