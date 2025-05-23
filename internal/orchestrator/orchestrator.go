package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"log"
	"log/slog"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sync"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	dockerClient "github.com/docker/docker/client"
	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
	"go.bug.st/f"
	"gopkg.in/yaml.v3"

	"github.com/arduino/arduino-app-cli/pkg/parser"
)

var (
	pythonImage        string
	orchestratorConfig *OrchestratorConfig

	ErrAppAlreadyExists = fmt.Errorf("app already exists")
	ErrAppDoesntExists  = fmt.Errorf("app doesn't exist")
	ErrInvalidApp       = fmt.Errorf("invalid app")
)

func init() {
	const dockerRegistry = "ghcr.io/bcmi-labs/"
	const dockerPythonImage = "arduino/appslab-python-apps-base:0.0.2"
	// Registry base: contains the registry and namespace, common to all Arduino docker images.
	registryBase := os.Getenv("DOCKER_REGISTRY_BASE")
	if registryBase == "" {
		registryBase = dockerRegistry
	}

	// Python image: image name (repository) and optionally a tag.
	pythonImageAndTag := os.Getenv("DOCKER_PYTHON_BASE_IMAGE")
	if pythonImageAndTag == "" {
		pythonImageAndTag = dockerPythonImage
	}

	pythonImage = path.Join(registryBase, pythonImageAndTag)
	slog.Debug("Using pythonImage", slog.String("image", pythonImage))

	// Load orchestrator OrchestratorConfig
	cfg, err := NewOrchestratorConfigFromEnv()
	if err != nil {
		panic(fmt.Errorf("failed to load orchestrator config: %w", err))
	}
	orchestratorConfig = cfg
}

type AppStreamMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type MessageType string

const (
	UnknownType  MessageType = ""
	ProgressType MessageType = "progress"
	InfoType     MessageType = "info"
	ErrorType    MessageType = "error"
)

type StreamMessage struct {
	data     string
	error    error
	progress *Progress
}

type Progress struct {
	Name     string
	Progress float32
}

func (p *StreamMessage) IsData() bool           { return p.data != "" }
func (p *StreamMessage) IsError() bool          { return p.error != nil }
func (p *StreamMessage) IsProgress() bool       { return p.progress != nil }
func (p *StreamMessage) GetData() string        { return p.data }
func (p *StreamMessage) GetError() error        { return p.error }
func (p *StreamMessage) GetProgress() *Progress { return p.progress }
func (p *StreamMessage) GetType() MessageType {
	if p.IsData() {
		return InfoType
	}
	if p.IsError() {
		return ErrorType
	}
	if p.IsProgress() {
		return ProgressType
	}
	return UnknownType
}

func StartApp(ctx context.Context, docker *dockerClient.Client, app parser.App) iter.Seq[StreamMessage] {
	return func(yield func(StreamMessage) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		running, err := getRunningApp(ctx, docker)
		if err != nil {
			yield(StreamMessage{error: err})
			return
		}
		if running != nil {
			yield(StreamMessage{error: fmt.Errorf("app %q is running", running.Name)})
			return
		}

		callbackWriter := NewCallbackWriter(func(line string) {
			if !yield(StreamMessage{data: line}) {
				cancel()
				return
			}
		})

		if app.MainSketchFile != nil {
			buildPath := app.FullPath.Join(".cache", "sketch").String()
			if err := compileUploadSketch(ctx, app.MainSketchFile.String(), buildPath, callbackWriter); err != nil {
				yield(StreamMessage{error: err})
				return
			}
		}
		if app.MainPythonFile != nil {
			if !yield(StreamMessage{data: "Provisioning app..."}) {
				cancel()
				return
			}
			if err := ProvisionApp(ctx, docker, app); err != nil {
				yield(StreamMessage{error: err})
				return
			}
			if !yield(StreamMessage{data: "Starting app..."}) {
				cancel()
				return
			}

			provisioningStateDir, err := getProvisioningStateDir(app)
			if err != nil {
				yield(StreamMessage{error: err})
				return
			}

			mainCompose := provisioningStateDir.Join("app-compose.yaml")
			process, err := paths.NewProcess(nil, "docker", "compose", "-f", mainCompose.String(), "up", "-d", "--remove-orphans")
			if err != nil {
				yield(StreamMessage{error: err})
				return
			}
			process.RedirectStderrTo(callbackWriter)
			process.RedirectStdoutTo(callbackWriter)
			if err := process.RunWithinContext(ctx); err != nil {
				yield(StreamMessage{error: err})
				return
			}
		}
		_ = yield(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	}
}

func StopApp(ctx context.Context, app parser.App) iter.Seq[StreamMessage] {
	return func(yield func(StreamMessage) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		callbackWriter := NewCallbackWriter(func(line string) {
			if !yield(StreamMessage{data: line}) {
				cancel()
				return
			}
		})
		if app.MainSketchFile != nil {
			// TODO: check that the app sketch is running before attempting to stop it.

			// Flash empty sketch to stop the microcontroller.
			buildPath := "" // the empty sketch' build path must be in the default temporary directory.
			if err := compileUploadSketch(ctx, getEmptySketch(), buildPath, callbackWriter); err != nil {
				panic(err)
			}
		}

		if app.MainPythonFile != nil {
			provisioningStateDir, err := getProvisioningStateDir(app)
			if err != nil {
				yield(StreamMessage{error: err})
				return
			}
			mainCompose := provisioningStateDir.Join("app-compose.yaml")
			// In case the app was never started
			if mainCompose.Exist() {
				process, err := paths.NewProcess(nil, "docker", "compose", "-f", mainCompose.String(), "stop")
				if err != nil {
					yield(StreamMessage{error: err})
					return
				}
				process.RedirectStderrTo(callbackWriter)
				process.RedirectStdoutTo(callbackWriter)
				if err := process.RunWithinContext(ctx); err != nil {
					yield(StreamMessage{error: err})
					return
				}
			}
		}
		_ = yield(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	}
}

func StartDefaultApp(ctx context.Context, docker *dockerClient.Client) error {
	app, err := GetDefaultApp()
	if err != nil {
		return fmt.Errorf("failed to get default app: %w", err)
	}
	if app == nil {
		// default app not set.
		return nil
	}

	status, err := AppDetails(ctx, *app)
	if err != nil {
		return fmt.Errorf("failed to get app details: %w", err)
	}
	if status.Status == "running" {
		return nil
	}

	// TODO: we need to stop all other running app before starting the default app.
	for msg := range StartApp(ctx, docker, *app) {
		if msg.IsError() {
			return fmt.Errorf("failed to start app: %w", msg.GetError())
		}
	}

	return nil
}

type ListAppResult struct {
	Apps []AppInfo `json:"apps"`
}

type AppInfo struct {
	ID          ID     `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Status      string `json:"status"` // TODO: create enum
	Example     bool   `json:"example"`
	Default     bool   `json:"default"`
}

type ListAppRequest struct {
	ShowExamples    bool
	ShowOnlyDefault bool
	StatusFilter    string // TODO: create enum
}

func ListApps(ctx context.Context, req ListAppRequest) (ListAppResult, error) {
	result := ListAppResult{Apps: []AppInfo{}}

	defaultApp, err := GetDefaultApp()
	if err != nil {
		slog.Warn("unable to get default app", slog.String("error", err.Error()))
	}

	var (
		pathsToExplore paths.PathList
		appPaths       paths.PathList
	)

	pathsToExplore.Add(orchestratorConfig.AppsDir())
	if req.ShowExamples {
		pathsToExplore.Add(orchestratorConfig.ExamplesDir())
	}
	for _, p := range pathsToExplore {
		res, err := p.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
			if file.Base() == ".cache" {
				return false
			}
			if file.Join("app.yaml").NotExist() && file.Join("app.yml").NotExist() {
				// Let's continue the scan, we might be in an parent folder
				return true
			}
			return false
		})
		if err != nil {
			slog.Error("unable to list apps", slog.String("error", err.Error()))
			return result, err
		}
		appPaths.AddAll(res)
	}

	for _, file := range appPaths {
		app, err := parser.Load(file.String())
		if err != nil {
			slog.Error("unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", file.String()))
			continue
		}

		isDefault := defaultApp != nil && defaultApp.FullPath.String() == app.FullPath.String()
		if req.ShowOnlyDefault && !isDefault {
			continue
		}

		resp, err := dockerComposeAppStatus(ctx, app)
		if err != nil {
			slog.Debug("unable to get app status", slog.String("error", err.Error()), slog.String("path", file.String()))
		}
		id, err := NewIDFromPath(app.FullPath)
		if err != nil {
			slog.Error("unable to get app id", slog.String("error", err.Error()), slog.String("path", file.String()))
			continue
		}

		if req.StatusFilter != "" && req.StatusFilter != resp.Status {
			continue
		}

		result.Apps = append(result.Apps,
			AppInfo{
				ID:          id,
				Name:        app.Name,
				Description: app.Descriptor.Description,
				Icon:        app.Descriptor.Icon,
				Status:      resp.Status,
				Example:     id.IsExample(),
				Default:     isDefault,
			},
		)
	}

	return result, nil
}

func AppDetails(ctx context.Context, app parser.App) (AppInfo, error) {
	var wg sync.WaitGroup
	wg.Add(2)
	var status, defaultAppPath string
	go func() {
		defer wg.Done()
		resp, err := dockerComposeAppStatus(ctx, app)
		if err != nil {
			slog.Warn("unable to get app status", slog.String("error", err.Error()), slog.String("path", app.FullPath.String()))
		}
		status = resp.Status
	}()
	go func() {
		defer wg.Done()
		defaultApp, err := GetDefaultApp()
		if err != nil {
			slog.Warn("unable to get default app", slog.String("error", err.Error()))
			return
		}
		if defaultApp == nil {
			return
		}
		defaultAppPath = defaultApp.FullPath.String()

	}()
	wg.Wait()

	id, err := NewIDFromPath(app.FullPath)
	if err != nil {
		return AppInfo{}, err
	}

	return AppInfo{
		ID:          id,
		Name:        app.Name,
		Description: app.Descriptor.Description,
		Icon:        app.Descriptor.Icon,
		Status:      status,
		Example:     id.IsExample(),
		Default:     defaultAppPath == app.FullPath.String(),
	}, nil
}

type CreateAppRequest struct {
	Name       string
	Icon       string
	Bricks     []string
	SkipPython bool
	SkipSketch bool
}

type CreateAppResponse struct {
	ID ID `json:"id"`
}

func CreateApp(ctx context.Context, req CreateAppRequest) (CreateAppResponse, error) {
	if req.SkipPython && req.SkipSketch {
		return CreateAppResponse{}, fmt.Errorf("cannot skip both python and sketch")
	}
	if req.Name == "" {
		return CreateAppResponse{}, fmt.Errorf("app name cannot be empty")
	}

	appFolderName := slug.Make(req.Name)
	basePath := orchestratorConfig.AppsDir().Join(appFolderName)
	if basePath.Exist() {
		return CreateAppResponse{}, ErrAppAlreadyExists
	}

	if err := basePath.MkdirAll(); err != nil {
		return CreateAppResponse{}, fmt.Errorf("failed to create app directory: %w", err)
	}
	if !req.SkipSketch {
		baseSketchPath := basePath.Join("sketch")
		if err := baseSketchPath.MkdirAll(); err != nil {
			return CreateAppResponse{}, fmt.Errorf("failed to create sketch directory: %w", err)
		}
		if err := baseSketchPath.Join("sketch.ino").WriteFile([]byte("void setup() {}\n\nvoid loop() {}")); err != nil {
			return CreateAppResponse{}, fmt.Errorf("failed to create sketch file: %w", err)
		}
		if err := baseSketchPath.Join("sketch.yaml").WriteFile([]byte("profiles:\n\ndefault_profile:")); err != nil {
			return CreateAppResponse{}, fmt.Errorf("failed to create sketch.yaml project file: %w", err)
		}
	}

	if !req.SkipPython {
		basePythonPath := basePath.Join("python")
		if err := basePythonPath.MkdirAll(); err != nil {
			return CreateAppResponse{}, fmt.Errorf("failed to create python directory: %w", err)
		}
		pythonContent := `def main():
    print("Hello World!")


if __name__ == "__main__":
    main()
`
		if err := basePythonPath.Join("main.py").WriteFile([]byte(pythonContent)); err != nil {
			return CreateAppResponse{}, fmt.Errorf("failed to create python file: %w", err)
		}
	}

	// TODO: create app yaml marshaler
	appContent := `name: "` + req.Name + `"` + "\n"
	if req.Icon != "" {
		appContent += `icon: "` + req.Icon + `"` + "\n"
	}
	if len(req.Bricks) > 0 {
		appContent += "module-dependencies:\n" // TODO: rename this when we update the parser. The spec was renamed to `dependencies`
		for _, brick := range req.Bricks {
			appContent += "  - " + brick + "\n"
		}
	}

	if err := basePath.Join("app.yaml").WriteFile([]byte(appContent)); err != nil {
		return CreateAppResponse{}, fmt.Errorf("failed to create app.yaml file: %w", err)
	}

	id, err := NewIDFromPath(basePath)
	if err != nil {
		return CreateAppResponse{}, fmt.Errorf("failed to get app id: %w", err)
	}
	return CreateAppResponse{ID: id}, nil
}

type CloneAppRequest struct {
	FromID ID

	Name *string
	Icon *string
}

type CloneAppResponse struct {
	ID ID `json:"id"`
}

func CloneApp(ctx context.Context, req CloneAppRequest) (response CloneAppResponse, cloneErr error) {
	originPath, err := req.FromID.ToPath()
	if err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to get app path: %w", err)
	}
	if !originPath.Exist() {
		return CloneAppResponse{}, ErrAppDoesntExists
	}
	if !originPath.Join("app.yaml").Exist() && !originPath.Join("app.yml").Exist() {
		return CloneAppResponse{}, ErrInvalidApp
	}

	var dstPath *paths.Path
	if req.Name != nil && *req.Name != "" {
		dstPath = orchestratorConfig.AppsDir().Join(slug.Make(*req.Name))
		if dstPath.Exist() {
			return CloneAppResponse{}, ErrAppAlreadyExists
		}
	} else {
		for i := range 100 { // In case of name collision, we try up to 100 times.
			dstName := fmt.Sprintf("%s-copy%d", originPath.Base(), i)
			dstPath = orchestratorConfig.AppsDir().Join(dstName)
			if !dstPath.Exist() {
				break
			}
		}
	}
	if err := dstPath.MkdirAll(); err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to create app directory: %w", err)
	}

	// In case something during the clone operation fails we remove the dst path
	defer func() {
		if cloneErr != nil {
			_ = dstPath.RemoveAll()
		}
	}()

	list, err := originPath.ReadDir(paths.FilterOutNames(".cache", "data"))
	if err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to read app directory: %w", err)
	}
	for _, file := range list {
		if file.IsDir() {
			if err := file.CopyDirTo(dstPath.Join(file.Base())); err != nil {
				return CloneAppResponse{}, fmt.Errorf("failed to copy directory: %w", err)
			}
		} else {
			if err := file.CopyTo(dstPath.Join(file.Base())); err != nil {
				return CloneAppResponse{}, fmt.Errorf("failed to copy file: %w", err)
			}
		}
	}

	if (req.Name != nil && *req.Name != "") || (req.Icon != nil && *req.Icon != "") {
		var appYamlPath *paths.Path
		if dstPath.Join("app.yaml").Exist() {
			appYamlPath = dstPath.Join("app.yaml")
		} else {
			appYamlPath = dstPath.Join("app.yml")
		}
		descriptor, err := parser.ParseDescriptorFile(appYamlPath)
		if err != nil {
			return CloneAppResponse{}, fmt.Errorf("failed to parse app.yaml file: %w", err)
		}
		if req.Name != nil && *req.Name != "" {
			descriptor.Name = *req.Name
		}
		if req.Icon != nil && *req.Icon != "" {
			descriptor.Icon = *req.Icon
		}

		// TODO: implement MarshalYaml directly in the descriptor.
		newDescriptor, err := yaml.Marshal(descriptor)
		if err != nil {
			// TODO: should we consider this a fatal error, or we prefer to silently ignore the error?
			// Worst case, the optional fields will be the same as the source app.
			return CloneAppResponse{}, fmt.Errorf("failed to marshal app.yaml file: %w", err)
		}
		if err := appYamlPath.WriteFile(newDescriptor); err != nil {
			return CloneAppResponse{}, fmt.Errorf("failed to write app.yaml file: %w", err)
		}
	}

	id, err := NewIDFromPath(dstPath)
	if err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to get app id: %w", err)
	}
	return CloneAppResponse{ID: id}, nil
}

func DeleteApp(ctx context.Context, app parser.App) error {
	for msg := range StopApp(ctx, app) {
		if msg.error != nil {
			return fmt.Errorf("failed to stop app: %w", msg.error)
		}
	}
	return app.FullPath.RemoveAll()
}

const defaultAppFileName = "default.app"

func SetDefaultApp(app *parser.App) error {
	defaultAppPath := orchestratorConfig.DataDir().Join(defaultAppFileName)

	// Remove the default app file if the app is nil.
	if app == nil {
		_ = defaultAppPath.Remove()
		return nil
	}

	f, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(app.FullPath.String())
	if err != nil {
		return err
	}
	f.Close()

	return os.Rename(f.Name(), defaultAppPath.String())
}

func GetDefaultApp() (*parser.App, error) {
	defaultAppFilePath := orchestratorConfig.DataDir().Join(defaultAppFileName)
	if !defaultAppFilePath.Exist() {
		return nil, nil
	}

	defaultAppPath, err := defaultAppFilePath.ReadFile()
	if err != nil {
		return nil, err
	}
	defaultAppPath = bytes.TrimSpace(defaultAppPath)
	if len(defaultAppPath) == 0 {
		// If the file is empty, we remove it
		slog.Warn("default app file is empty", slog.String("path", string(defaultAppPath)))
		_ = defaultAppFilePath.Remove()
		return nil, nil
	}

	app, err := parser.Load(string(defaultAppPath))
	if err != nil {
		// If the app is not valid, we remove the file
		slog.Warn("default app is not valid", slog.String("path", string(defaultAppPath)), slog.String("error", err.Error()))
		_ = defaultAppFilePath.Remove()
		return nil, err
	}

	return &app, nil
}

func getCurrentUser() string {
	// MacOS and Windows uses a VM so we don't need to map the user.
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return ""
	}
	// Map user to avoid permission issues.
	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	return user.Uid + ":" + user.Gid
}

func getDevices() []string {
	// Ignore devices on Windows
	if runtime.GOOS == "windows" {
		return nil
	}

	deviceList, err := paths.New("/dev").ReadDir()
	if err != nil {
		panic(err)
	}
	deviceList.FilterPrefix("video")
	return deviceList.AsStrings()
}

func compileUploadSketch(ctx context.Context, sketchPath, buildPath string, w io.Writer) error {
	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return err
	} else {
		inst = resp.GetInstance()
	}

	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	sketchResp, err := srv.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath})
	if err != nil {
		return err
	}
	sketch := sketchResp.GetSketch()
	initReq := &rpc.InitRequest{Instance: inst, SketchPath: sketchPath}
	if profile := sketch.GetDefaultProfile().GetName(); profile != "" {
		initReq.Profile = profile
	}
	if err := srv.Init(
		initReq,
		// TODO: implement progress callback function
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error { return nil }),
	); err != nil {
		return err
	}

	resp, err := srv.BoardList(ctx, &rpc.BoardListRequest{
		Instance:                      inst,
		Timeout:                       1000,
		Fqbn:                          "",
		SkipCloudApiForBoardDetection: false,
	})
	if err != nil {
		return err
	}

	idx := slices.IndexFunc(resp.Ports, func(p *rpc.DetectedPort) bool {
		return len(p.MatchingBoards) > 0
	})
	if idx == -1 {
		return fmt.Errorf("no board detected")
	}

	name := resp.Ports[idx].MatchingBoards[0].Name
	fqbn := resp.Ports[idx].MatchingBoards[0].Fqbn
	port := resp.Ports[idx].Port
	fmt.Println("\nAuto selected board:", name, "fqbn:", fqbn, "port:", port.Address)

	// build the sketch
	server, getCompileResult := commands.CompilerServerToStreams(ctx, w, w, nil)

	// TODO: add build cache
	err = srv.Compile(&rpc.CompileRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: sketchPath,
		BuildPath:  buildPath,
		Libraries:  []string{sketchPath + "/../../sketch-libraries"},
	}, server)
	if err != nil {
		return err
	}

	// Output compilations details
	result := getCompileResult()
	f.Assert(result != nil, "Failed to get compilation result")
	// TODO: maybe handle result.GetDiagnostics()
	boardPlatform := result.GetBoardPlatform()
	if boardPlatform != nil {
		slog.Info("Board platform: " + boardPlatform.GetId() + " (" + boardPlatform.GetVersion() + ") in " + boardPlatform.GetInstallDir())
	}
	buildPlatform := result.GetBuildPlatform()
	if buildPlatform != nil && buildPlatform.GetInstallDir() != boardPlatform.GetInstallDir() {
		slog.Info("Build platform: " + buildPlatform.GetId() + " (" + buildPlatform.GetVersion() + ") in " + buildPlatform.GetInstallDir())
	}
	for _, lib := range result.GetUsedLibraries() {
		slog.Info("Used library " + lib.GetName() + " (" + lib.GetVersion() + ") in " + lib.GetInstallDir())
	}

	stream, _ := commands.UploadToServerStreams(ctx, w, w)
	err = srv.Upload(&rpc.UploadRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: sketchPath,
		Port:       port,
	}, stream)
	if err != nil {
		return err
	}

	return nil
}

func getEmptySketch() string {
	const emptySketch = `void setup() {}
void loop() {}
`
	dir := filepath.Join(os.TempDir(), "empty_sketch")
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Panic(err)
	}
	ino := filepath.Join(dir, "empty_sketch.ino")
	err := os.WriteFile(ino, []byte(emptySketch), 0600)
	if err != nil {
		panic(err)
	}
	return ino
}
