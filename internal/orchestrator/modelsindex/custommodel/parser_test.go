// This file is part of arduino-app-cli.
//
// Copyright Copyright (C) Arduino s.r.l. and/or its affiliated companies
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

package custommodel

import (
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestParseModelDescription(t *testing.T) {
	modelDescriptor := `
id: "my-model-id"
name: "my custom model name"
runner: "bricks"
description: "A small and accurate description."
bricks:
  - id: "arduino:a-brick-id"
    model_configuration:
      "A_STRING_VARIABLE": "i-am-a-string"
      "A_BOOL_VARIABLE": true
  - id: "arduino:another-brick-id"
    model_configuration:
      "A_STRING_VARIABLE": "i-am-a-string"
      "A_BOOL_VARIABLE": false
metadata:
  a-string-metadata: "a-string-value"
  a-int-metadata: 717280
`
	modelYamlPath := paths.New(t.TempDir(), "model.yaml")
	err := os.WriteFile(modelYamlPath.String(), []byte(modelDescriptor), 0600)
	require.NoError(t, err)

	descr, err := ParseModelDescriptorFile(modelYamlPath)
	require.NoError(t, err)

	require.Equal(t, ModelDescriptor{
		ID:          "my-model-id",
		Name:        "my custom model name",
		Runner:      "bricks",
		Description: "A small and accurate description.",
		Bricks: []BrickConfig{
			{
				ID: "arduino:a-brick-id",
				ModelConfiguration: map[string]string{
					"A_STRING_VARIABLE": "i-am-a-string",
					"A_BOOL_VARIABLE":   "true",
				},
			},
			{
				ID: "arduino:another-brick-id",
				ModelConfiguration: map[string]string{
					"A_STRING_VARIABLE": "i-am-a-string",
					"A_BOOL_VARIABLE":   "false",
				},
			},
		},
		Metadata: map[string]string{
			"a-string-metadata": "a-string-value",
			"a-int-metadata":    "717280",
		},
	}, descr)

}
