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
