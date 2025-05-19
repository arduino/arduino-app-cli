package orchestrator

import (
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

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	dockerClient "github.com/docker/docker/client"
	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
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

		callbackWriter := NewCallbackWriter(func(line string) {
			if !yield(StreamMessage{data: line}) {
				cancel()
				return
			}
		})

		if app.MainSketchFile != nil {
			if err := compileUploadSketch(ctx, app.MainSketchFile.String(), callbackWriter); err != nil {
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
			// Flash empty sketch to stop the microcontroller.
			// TODO: check that the app sketch is running before attempting to stop it.
			if err := compileUploadSketch(ctx, getEmptySketch(), callbackWriter); err != nil {
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

type ListAppResult struct {
	Apps []AppInfo `json:"apps"`
}

type AppInfo struct {
	ID          ID       `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    []string `json:"category"`
	Icon        string   `json:"icon"`
	Status      string   `json:"status"` // TODO: create enum
	Example     bool     `json:"example"`
}

func ListApps(ctx context.Context) (ListAppResult, error) {
	result := ListAppResult{Apps: []AppInfo{}}

	filterFunc := func(file *paths.Path) bool {
		if file.Join("app.yaml").Exist() || file.Join("app.yml").Exist() {
			app, err := parser.Load(file.String())
			if err != nil {
				slog.Error("unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", file.String()))
				return false
			}

			var status string
			resp, err := dockerComposeAppStatus(ctx, app)
			if err != nil {
				slog.Warn("unable to get app status", slog.String("error", err.Error()), slog.String("path", file.String()))
			}
			id, err := NewIDFromPath(app.FullPath)
			if err != nil {
				slog.Error("unable to get app id", slog.String("error", err.Error()), slog.String("path", file.String()))
				return false
			}
			status = resp.Status
			result.Apps = append(result.Apps,
				AppInfo{
					ID:          id,
					Name:        app.Name,
					Description: app.Descriptor.Description,
					Category:    []string{}, // TODO: add category on parser
					Icon:        "",         // TODO: add icon on parser
					Status:      status,
					Example:     id.IsExample(),
				},
			)
		}
		return false
	}

	for _, p := range []*paths.Path{orchestratorConfig.AppsDir(), orchestratorConfig.ExamplesDir()} {
		_, err := p.ReadDirRecursiveFiltered(paths.FilterDirectories(), filterFunc)
		if err != nil {
			slog.Error("unable to list apps", slog.String("error", err.Error()))
			return result, err
		}
	}
	return result, nil
}

type AppDetailsResult struct {
	ID          ID       `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    []string `json:"category"`
	Icon        string   `json:"icon"`
	Status      string   `json:"status"` // TODO: create enum
	Example     bool     `json:"example"`
}

func AppDetails(ctx context.Context, app parser.App) (AppDetailsResult, error) {
	result := AppDetailsResult{}

	var status string
	resp, err := dockerComposeAppStatus(ctx, app)
	if err != nil {
		slog.Warn("unable to get app status", slog.String("error", err.Error()), slog.String("path", app.FullPath.String()))
	}
	status = resp.Status

	id, err := NewIDFromPath(app.FullPath)
	if err != nil {
		return result, err
	}

	result.Status = status
	result.ID = id
	result.Name = app.Name
	result.Description = app.Descriptor.Description
	result.Category = []string{} // TODO: add category on parser
	result.Icon = ""             // TODO: add icon on parser
	result.Example = result.ID.IsExample()

	return result, nil
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
	appContent := `display-name: "` + req.Name + `"` + "\n"
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
			descriptor.DisplayName = *req.Name
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

func compileUploadSketch(ctx context.Context, path string, w io.Writer) error {
	logrus.SetLevel(logrus.ErrorLevel)
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

	if err := srv.Init(
		&rpc.InitRequest{Instance: inst},
		commands.InitStreamResponseToCallbackFunction(ctx, nil),
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
	server, _ := commands.CompilerServerToStreams(ctx, w, w, nil)

	// TODO: add build cache
	// TODO: maybe handle resultCB.GetDiagnostics()
	err = srv.Compile(&rpc.CompileRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: path,
	}, server)
	if err != nil {
		return err
	}

	stream, _ := commands.UploadToServerStreams(ctx, w, w)
	err = srv.Upload(&rpc.UploadRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: path,
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
