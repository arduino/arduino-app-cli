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

package daemon

import (
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

const (
	ImageClassifactionBrickID      = "arduino:image_classification"
	StreamLitUi                    = "arduino:streamlit_ui"
	expectedDetailsAppNotfound     = "unable to find the app"
	expectedDetailsAppInvalidAppId = "invalid app id"
	noExistingApp                  = "dXNlcjp0ZXN0LWFwcAw"
	malformedAppId                 = "this-is-definitely-not-base64"
	noExisitingExample             = "ZXhhbXBsZXM6anVzdGJsaW5f"
)

var (
	expectedVariablesDetails = []client.BrickInstanceVariable{
		{
			Description: f.Ptr("path to the custom model directory"),
			Name:        f.Ptr("CUSTOM_MODEL_PATH"),
			Required:    f.Ptr(false),
			Value:       f.Ptr(""),
		},
		{
			Description: f.Ptr("path to the model file"),
			Name:        f.Ptr("EI_CLASSIFICATION_MODEL"),
			Required:    f.Ptr(false),
			Value:       f.Ptr(""),
		},
	}
)
