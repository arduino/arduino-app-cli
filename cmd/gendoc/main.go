// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
