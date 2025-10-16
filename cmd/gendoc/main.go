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

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	err := RunGenDoc("internal/api/docs/openapi.yaml")
	if err != nil {
		panic(err)
	}
}

// TODO add version on NewOpenApiGenerator
func RunGenDoc(outputPath string) error {
	docGenerator := NewOpenApiGenerator("0.1.0")
	docGenerator.InitOperations()

	yamlBytes, err := docGenerator.GetDocs().MarshalYAML()
	if err != nil {
		return err
	}

	outputDir := filepath.Dir(outputPath)
	if err = os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	err = os.WriteFile(outputPath, yamlBytes, 0600)
	if err != nil {
		return err
	}
	fmt.Printf("File OpenAPI generated and stored on path: %q\n", outputPath)

	return nil
}
