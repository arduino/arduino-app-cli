package generator

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

var update = flag.Bool("update", false, "update golden files")

func TestGenerateSketch(t *testing.T) {

	tempDir := t.TempDir()
	err := generateSketch(paths.New(tempDir))
	require.NoError(t, err, "generateSketch should run without errors")

	testCases := []struct {
		filename string
	}{
		{filename: "sketch.ino"},
		{filename: "sketch.yaml"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {

			actualFilePath := filepath.Join(tempDir, "sketch", tc.filename)
			require.FileExists(t, actualFilePath)
			actualContent, err := os.ReadFile(actualFilePath)
			require.NoError(t, err)

			goldenFilePath := filepath.Join("testdata", tc.filename+".golden")

			if *update {
				err := os.WriteFile(goldenFilePath, actualContent, 0600)
				require.NoError(t, err, "failed to update golden file: %s", goldenFilePath)
			}

			expectedContent, err := os.ReadFile(goldenFilePath)
			require.NoError(t, err, "failed to read golden file: %s", goldenFilePath)

			require.Equal(t, string(expectedContent), string(actualContent), "the generated content does not match the .golden file")
		})
	}
}

func TestGeneratePython(t *testing.T) {
	tempDir := t.TempDir()

	err := generatePython(paths.New(tempDir))
	require.NoError(t, err, "GeneratePython() should not return an error")

	verifyPythonGenerated(t, tempDir)
}

func TestGenerateReadme(t *testing.T) {
	tempDir := t.TempDir()
	testApp := app.AppDescriptor{
		Name:        "test name",
		Description: "test description.",
		Icon:        "🚀",
		Ports:       []int{8080, 9000},
	}

	err := generateReadme(paths.New(tempDir), testApp)
	require.NoError(t, err, "GenerateReadme() should not return an error")

	verifyReadmeGenerated(t, testApp, tempDir)

}

func TestGenerateAppYaml(t *testing.T) {
	tempDir := t.TempDir()

	testApp := app.AppDescriptor{
		Name:        "test name",
		Description: "test description.",
		Icon:        "🚀",
		Ports:       []int{8080, 9000, 90},
	}

	err := generateAppYaml(paths.New(tempDir), testApp)
	require.NoError(t, err, "GenerateAppYaml() should not return an error")

	yamlPath := filepath.Join(tempDir, "app.yaml")
	require.FileExists(t, yamlPath, "The app.yaml file should exist")

	actualContent, err := os.ReadFile(yamlPath)
	require.NoError(t, err, "Failed to read the generated app.yaml file")

	contentString := string(actualContent)
	require.Contains(t, contentString, "name: test name", "The YAML should contain the correct name")
	require.Contains(t, contentString, `description: "test description."`, "The YAML should contain the correct description")
	require.Contains(t, contentString, "icon: 🚀", "The YAML should contain the correct icon")
	require.Contains(t, contentString, "ports: [8080, 9000, 90]", "The YAML should contain the correct ports list"+contentString)
	require.Contains(t, contentString, "bricks: []", "The YAML should contain an empty bricks list")

}

func TestGenerateAndParseAppYaml_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()

	testApp := app.AppDescriptor{
		Name:        "test name",
		Description: "test description.",
		Icon:        "🚀",
		Ports:       []int{8080, 9000, 90},
		Bricks:      []app.Brick{},
	}

	err := generateAppYaml(paths.New(tempDir), testApp)
	require.NoError(t, err, "Step 1: Generation should not fail")

	parsedApp, err := app.ParseDescriptorFile(paths.New(tempDir, "app.yaml"))
	require.NoError(t, err, "Parsing the generated file should not fail")
	require.Equal(t, testApp, parsedApp, "The parsed app should be identical to the original one")
}

func TestGeneratCompleteApp(t *testing.T) {
	tempDir := t.TempDir()

	testApp := app.AppDescriptor{
		Name:        "test name",
		Description: "test description.",
		Icon:        "🚀",
		Ports:       []int{8080, 9000, 90},
		Bricks:      []app.Brick{},
	}

	var options Opts = 0

	err := GenerateApp(paths.New(tempDir), testApp, options)
	require.NoError(t, err, "GenerateApp() should not return an error")

	parsedApp, err := app.ParseDescriptorFile(paths.New(tempDir, "app.yaml"))
	require.NoError(t, err, "Parsing the generated file should not fail")
	require.Equal(t, testApp, parsedApp, "The parsed app should be identical to the original one")

	verifyReadmeGenerated(t, testApp, tempDir)
	verifySketchGenerated(t, tempDir)
	verifyPythonGenerated(t, tempDir)

}

func TestGeneratAppSkipPython(t *testing.T) {
	tempDir := t.TempDir()

	testApp := app.AppDescriptor{
		Name:        "test name",
		Description: "test description.",
		Icon:        "🚀",
		Ports:       []int{8080, 9000, 90},
		Bricks:      []app.Brick{},
	}
	var options Opts = 0
	options |= SkipPython

	err := GenerateApp(paths.New(tempDir), testApp, options)
	require.NoError(t, err, "GenerateApp() should not return an error")

	parsedApp, err := app.ParseDescriptorFile(paths.New(tempDir, "app.yaml"))
	require.NoError(t, err, "Parsing the generated file should not fail")
	require.Equal(t, testApp, parsedApp, "The parsed app should be identical to the original one")

	verifyReadmeGenerated(t, testApp, tempDir)
	verifySketchGenerated(t, tempDir)

	pythonPath := filepath.Join(tempDir, "python")
	require.NoDirExists(t, pythonPath, "The 'python' directory should NOT exist when skipped")
}

func TestGeneratAppSkipSketch(t *testing.T) {
	tempDir := t.TempDir()

	testApp := app.AppDescriptor{
		Name:        "test name",
		Description: "test description.",
		Icon:        "🚀",
		Ports:       []int{8080, 9000, 90},
		Bricks:      []app.Brick{},
	}
	var options Opts = 0
	options |= SkipSketch

	err := GenerateApp(paths.New(tempDir), testApp, options)
	require.NoError(t, err, "GenerateApp() should not return an error")

	parsedApp, err := app.ParseDescriptorFile(paths.New(tempDir, "app.yaml"))
	require.NoError(t, err, "Parsing the generated file should not fail")
	require.Equal(t, testApp, parsedApp, "The parsed app should be identical to the original one")

	verifyReadmeGenerated(t, testApp, tempDir)
	verifyPythonGenerated(t, tempDir)

	sketchPath := filepath.Join(tempDir, "sketch")
	require.NoDirExists(t, sketchPath, "The 'sketch' directory should NOT exist when skipped")
}

func TestGeneratAppSkipBoth(t *testing.T) {
	tempDir := t.TempDir()

	testApp := app.AppDescriptor{
		Name:        "test name",
		Description: "test description.",
		Icon:        "🚀",
		Ports:       []int{8080, 9000, 90},
		Bricks:      []app.Brick{},
	}
	var options Opts = 0
	options |= SkipSketch
	options |= SkipPython

	err := GenerateApp(paths.New(tempDir), testApp, options)
	require.NoError(t, err, "GenerateApp() should not return an error")

	parsedApp, err := app.ParseDescriptorFile(paths.New(tempDir, "app.yaml"))
	require.NoError(t, err, "Parsing the generated file should not fail")
	require.Equal(t, testApp, parsedApp, "The parsed app should be identical to the original one")

	verifyReadmeGenerated(t, testApp, tempDir)

	pythonPath := filepath.Join(tempDir, "python")
	require.NoDirExists(t, pythonPath, "The 'python' directory should NOT exist when skipped")

	sketchPath := filepath.Join(tempDir, "sketch")
	require.NoDirExists(t, sketchPath, "The 'sketch' directory should NOT exist when skipped")
}

func verifySketchGenerated(t *testing.T, basePath string) {
	t.Helper()

	sketchPath := filepath.Join(basePath, "sketch")
	inoPath := filepath.Join(sketchPath, "sketch.ino")
	yamlPath := filepath.Join(sketchPath, "sketch.yaml")

	require.DirExists(t, sketchPath, "The 'sketch' directory should exist")
	require.FileExists(t, inoPath, "The 'sketch.ino' file should exist")
	require.FileExists(t, yamlPath, "The 'sketch.yaml' file should exist")

	expectedInoContent, err := fsApp.ReadFile("app_template/sketch/sketch.ino")
	require.NoError(t, err)
	actualInoContent, err := os.ReadFile(inoPath)
	require.NoError(t, err)
	require.Equal(t, string(expectedInoContent), string(actualInoContent))

	expectedYamlContent, err := fsApp.ReadFile("app_template/sketch/sketch.yaml")
	require.NoError(t, err)
	actualYamlContent, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	require.Equal(t, string(expectedYamlContent), string(actualYamlContent))
}

func verifyPythonGenerated(t *testing.T, basePath string) {
	t.Helper()

	pythonPath := filepath.Join(basePath, "python")
	mainPath := filepath.Join(pythonPath, "main.py")

	require.DirExists(t, pythonPath, "The 'python' directory should exist")
	require.FileExists(t, mainPath, "The 'main.py' file should exist")

	expectedMainContent, err := fsApp.ReadFile("app_template/python/main.py")
	require.NoError(t, err)
	actualMainContent, err := os.ReadFile(mainPath)
	require.NoError(t, err)
	require.Equal(t, string(expectedMainContent), string(actualMainContent))
}

func verifyReadmeGenerated(t *testing.T, testApp app.AppDescriptor, basePath string) {
	t.Helper()

	readmePath := filepath.Join(basePath, "README.md")
	require.FileExists(t, readmePath, "The README.md file should exist")

	actualContent, err := os.ReadFile(readmePath)
	require.NoError(t, err, "Failed to read the generated README.md file")

	contentString := string(actualContent)
	require.Contains(t, contentString, testApp.Name, "The README should contain the app name")
	require.Contains(t, contentString, testApp.Description, "The README should contain the app description")
	require.Contains(t, contentString, testApp.Icon, "The README should contain the app icon")
	require.Contains(t, contentString, "8080, 9000", "The README should contain the formatted port list")
	require.Contains(t, contentString, "Available application ports:", "The README should contain the static text for ports")
}
