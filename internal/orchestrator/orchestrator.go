package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net"
	"os"
	"os/user"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	"github.com/goccy/go-yaml"
	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/cmd/arduino-router/msgpackrpc"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/pkg/helpers"
	"github.com/arduino/arduino-app-cli/pkg/micro"
	"github.com/arduino/arduino-app-cli/pkg/x/fatomic"
)

var (
	orchestratorConfig *OrchestratorConfig
)

var (
	ErrAppAlreadyExists = fmt.Errorf("app already exists")
	ErrAppDoesntExists  = fmt.Errorf("app doesn't exist")
	ErrInvalidApp       = fmt.Errorf("invalid app")
)

const (
	DefaultDockerStopTimeoutSeconds = 5
)

func init() {
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

func StartApp(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	app app.ArduinoApp,
) iter.Seq[StreamMessage] {
	return func(yield func(StreamMessage) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		running, err := getRunningApp(ctx, docker.Client())
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

		if app.MainSketchPath != nil {
			if !yield(StreamMessage{data: "compiling and updating sketch..."}) {
				cancel()
				return
			}
			if err := compileUploadSketch(ctx, &app, callbackWriter); err != nil {
				yield(StreamMessage{error: err})
				return
			}
		}
		if app.MainPythonFile != nil {
			if !yield(StreamMessage{data: "Provisioning app..."}) {
				cancel()
				return
			}
			if err := ProvisionApp(ctx, provisioner, bricksIndex, &app); err != nil {
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

			// Override the compose Variables with the app's variables and model configuration.
			envs := []string{}
			addMapToEnv := func(m map[string]string) {
				for k, v := range m {
					envs = append(envs, fmt.Sprintf("%s=%s", k, v))
				}
			}
			for _, brick := range app.Descriptor.Bricks {
				addMapToEnv(brick.Variables)
				if m, found := modelsIndex.GetModelByID(brick.Model); found {
					addMapToEnv(m.ModelConfiguration)
				}
			}

			mainCompose := provisioningStateDir.Join("app-compose.yaml")
			overrideComposeFile := provisioningStateDir.Join("app-compose-overrides.yaml")
			commands := []string{}
			commands = append(commands, "docker", "compose", "-f", mainCompose.String())
			if ok, _ := overrideComposeFile.ExistCheck(); ok {
				commands = append(commands, "-f", overrideComposeFile.String())
			}
			commands = append(commands, "up", "-d", "--remove-orphans", "--pull", "missing")
			process, err := paths.NewProcess(envs, commands...)
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

func StopApp(ctx context.Context, app app.ArduinoApp) iter.Seq[StreamMessage] {
	return func(yield func(StreamMessage) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		callbackWriter := NewCallbackWriter(func(line string) {
			if !yield(StreamMessage{data: line}) {
				cancel()
				return
			}
		})
		if app.MainSketchPath != nil {
			// TODO: check that the app sketch is running before attempting to stop it.

			if micro.OnBoard {
				// On imola we could just disable the microcontroller
				if err := micro.Disable(context.Background(), nil); err != nil {
					yield(StreamMessage{error: err})
					return
				}
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
				process, err := paths.NewProcess(nil, "docker", "compose", "-f", mainCompose.String(), "stop", fmt.Sprintf("--timeout=%d", DefaultDockerStopTimeoutSeconds))
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

func StartDefaultApp(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) error {
	app, err := GetDefaultApp()
	if err != nil {
		return fmt.Errorf("failed to get default app: %w", err)
	}
	if app == nil {
		// default app not set.
		return nil
	}

	status, err := AppDetails(ctx, docker, *app, bricksIndex)
	if err != nil {
		return fmt.Errorf("failed to get app details: %w", err)
	}
	if status.Status == "running" {
		return nil
	}

	// TODO: we need to stop all other running app before starting the default app.
	for msg := range StartApp(ctx, docker, provisioner, modelsIndex, bricksIndex, *app) {
		if msg.IsError() {
			return fmt.Errorf("failed to start app: %w", msg.GetError())
		}
	}

	return nil
}

type ListAppResult struct {
	Apps       []AppInfo       `json:"apps"`
	BrokenApps []BrokenAppInfo `json:"broken_apps"`
}

type AppInfo struct {
	ID          ID     `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Status      Status `json:"status,omitempty"`
	Example     bool   `json:"example"`
	Default     bool   `json:"default"`
}

type BrokenAppInfo struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type ListAppRequest struct {
	ShowExamples    bool
	ShowOnlyDefault bool
	ShowApps        bool
	StatusFilter    Status

	// IncludeNonStandardLocationApps will include apps that are not in the standard apps directory.
	// We will search by looking for docker container metadata, and add the app not present in the
	// standard apps directory in the result list.
	IncludeNonStandardLocationApps bool
}

func ListApps(
	ctx context.Context,
	docker command.Cli,
	req ListAppRequest,
) (ListAppResult, error) {
	var (
		pathsToExplore paths.PathList
		appPaths       paths.PathList
	)

	apps, err := getAppsStatus(ctx, docker.Client())
	if err != nil {
		slog.Error("unable to get running app", slog.String("error", err.Error()))
	}

	if req.ShowExamples {
		pathsToExplore.Add(orchestratorConfig.ExamplesDir())
	}
	if req.ShowApps {
		pathsToExplore.Add(orchestratorConfig.AppsDir())
		// adds app that are on different paths
		if req.IncludeNonStandardLocationApps {
			for _, app := range apps {
				appPaths.AddIfMissing(app.AppPath)
			}
		}
	}

	result := ListAppResult{Apps: []AppInfo{}, BrokenApps: []BrokenAppInfo{}}
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
		}, paths.FilterDirectories(), paths.FilterOutNames("python", "sketch", ".cache"))
		if err != nil {
			slog.Error("unable to list apps", slog.String("error", err.Error()))
			return result, err
		}
		appPaths.AddAllMissing(res)
	}

	defaultApp, err := GetDefaultApp()
	if err != nil {
		slog.Warn("unable to get default app", slog.String("error", err.Error()))
	}

	for _, file := range appPaths {
		app, err := app.Load(file.String())
		if err != nil {
			result.BrokenApps = append(result.BrokenApps, BrokenAppInfo{
				Name:  file.Base(),
				Error: fmt.Sprintf("unable to parse the app.yaml: %s", err.Error()),
			})
			continue
		}

		isDefault := defaultApp != nil && defaultApp.FullPath.String() == app.FullPath.String()
		if req.ShowOnlyDefault && !isDefault {
			continue
		}

		var status Status
		if idx := slices.IndexFunc(apps, func(a AppStatus) bool {
			return a.AppPath.EqualsTo(app.FullPath)
		}); idx != -1 {
			status = apps[idx].Status
		}

		if req.StatusFilter != "" && req.StatusFilter != status {
			continue
		}

		id, err := NewIDFromPath(app.FullPath)
		if err != nil {
			return ListAppResult{}, fmt.Errorf("failed to get app ID from path %s: %w", file.String(), err)
		}

		result.Apps = append(result.Apps,
			AppInfo{
				ID:          id,
				Name:        app.Name,
				Description: app.Descriptor.Description,
				Icon:        app.Descriptor.Icon,
				Status:      status,
				Example:     id.IsExample(),
				Default:     isDefault,
			},
		)
	}

	return result, nil
}

type AppDetailedInfo struct {
	ID          ID                 `json:"id" required:"true" `
	Name        string             `json:"name" required:"true"`
	Path        string             `json:"path"`
	Description string             `json:"description"`
	Icon        string             `json:"icon"`
	Status      Status             `json:"status" required:"true"`
	Example     bool               `json:"example"`
	Default     bool               `json:"default"`
	Bricks      []AppDetailedBrick `json:"bricks,omitempty"`
}

type AppDetailedBrick struct {
	ID       string `json:"id" required:"true"`
	Name     string `json:"name" required:"true"`
	Category string `json:"category,omitempty"`
}

func AppDetails(
	ctx context.Context,
	docker command.Cli,
	userApp app.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
) (AppDetailedInfo, error) {
	var wg sync.WaitGroup
	wg.Add(2)
	var defaultAppPath string
	var status Status
	go func() {
		defer wg.Done()
		app, err := getAppStatus(ctx, docker, userApp)
		if err != nil {
			slog.Warn("unable to get app status", slog.String("error", err.Error()), slog.String("path", userApp.FullPath.String()))
			status = StatusStopped
		} else {
			status = app.Status
		}
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

	id, err := NewIDFromPath(userApp.FullPath)
	if err != nil {
		return AppDetailedInfo{}, err
	}

	return AppDetailedInfo{
		ID:          id,
		Name:        userApp.Name,
		Path:        userApp.FullPath.String(),
		Description: userApp.Descriptor.Description,
		Icon:        userApp.Descriptor.Icon,
		Status:      status,
		Example:     id.IsExample(),
		Default:     defaultAppPath == userApp.FullPath.String(),
		Bricks: f.Map(userApp.Descriptor.Bricks, func(b app.Brick) AppDetailedBrick {
			res := AppDetailedBrick{ID: b.ID}
			bi, found := bricksIndex.FindBrickByID(b.ID)
			if !found {
				slog.Warn("brick not found in bricks index", slog.String("id", b.ID), slog.String("app", userApp.FullPath.String()))
				return res
			}
			res.Name = bi.Name
			res.Category = bi.Category
			return res
		}),
	}, nil
}

type CreateAppRequest struct {
	Name       string
	Icon       string
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

	basePath, appExists := findAppPathByName(req.Name)
	if appExists {
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

	appYaml, err := yaml.Marshal(
		app.AppDescriptor{
			Name:        req.Name,
			Description: "",
			Ports:       []int{},
			Icon:        req.Icon, // TODO: not sure if icon will exists for bricks
		},
	)
	if err != nil {
		return CreateAppResponse{}, fmt.Errorf("failed to marshal app.yaml content: %w", err)
	}

	if err := basePath.Join("app.yaml").WriteFile(appYaml); err != nil {
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
	originPath := req.FromID.ToPath()
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
		descriptor, err := app.ParseDescriptorFile(appYamlPath)
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

func DeleteApp(ctx context.Context, app app.ArduinoApp) error {
	for msg := range StopApp(ctx, app) {
		if msg.error != nil {
			return fmt.Errorf("failed to stop app: %w", msg.error)
		}
	}
	return app.FullPath.RemoveAll()
}

const defaultAppFileName = "default.app"

func SetDefaultApp(app *app.ArduinoApp) error {
	defaultAppPath := orchestratorConfig.DataDir().Join(defaultAppFileName)

	// Remove the default app file if the app is nil.
	if app == nil {
		err := defaultAppPath.Remove()
		if err != nil {
			slog.Warn("failed to remove default app file", slog.String("path", defaultAppPath.String()), slog.String("error", err.Error()))
		}
		return nil
	}

	return fatomic.WriteFile(defaultAppPath.String(), []byte(app.FullPath.String()), os.FileMode(0644))
}

func GetDefaultApp() (*app.ArduinoApp, error) {
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

	app, err := app.Load(string(defaultAppPath))
	if err != nil {
		// If the app is not valid, we remove the file
		slog.Warn("default app is not valid", slog.String("path", string(defaultAppPath)), slog.String("error", err.Error()))
		_ = defaultAppFilePath.Remove()
		return nil, err
	}

	return &app, nil
}

type AppEditRequest struct {
	Name        *string
	Icon        *string
	Description *string
	Default     *bool
}

func EditApp(req AppEditRequest, app *app.ArduinoApp) (editErr error) {
	if req.Default != nil {
		if err := editAppDefaults(app, *req.Default); err != nil {
			return fmt.Errorf("failed to edit app defaults: %w", err)
		}
	}

	if req.Name != nil {
		app.Descriptor.Name = *req.Name
		newPath := app.FullPath.Parent().Join(slug.Make(*req.Name))
		if newPath.Exist() {
			return ErrAppAlreadyExists
		}
		if err := app.FullPath.Rename(newPath); err != nil {
			editErr = fmt.Errorf("failed to rename app path: %w", err)
			return editErr
		}
		app.FullPath = newPath
		app.Name = app.Descriptor.Name
	}

	if req.Icon != nil {
		app.Descriptor.Icon = *req.Icon
	}
	if req.Description != nil {
		app.Descriptor.Description = *req.Description
	}

	err := app.Save()
	if err != nil {
		return fmt.Errorf("failed to save app: %w", err)
	}
	return nil
}

func editAppDefaults(userApp *app.ArduinoApp, isDefault bool) error {
	if isDefault {
		if err := SetDefaultApp(userApp); err != nil {
			return fmt.Errorf("failed to set default app: %w", err)
		}
		return nil
	}

	defaultApp, err := GetDefaultApp()
	if err != nil {
		return fmt.Errorf("failed to get default app: %w", err)
	}

	// No default app set, nothing to unset.
	if defaultApp == nil {
		return nil
	}

	// Unset only if the current default is the same as the app being edited.
	if defaultApp.FullPath.String() == userApp.FullPath.String() {
		if err := SetDefaultApp(nil); err != nil {
			return fmt.Errorf("failed to unset default app: %w", err)
		}
	}
	return nil
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

	// Get video devices
	tmpFilter := deviceList
	tmpFilter.FilterPrefix("video")
	videoDevices := tmpFilter.AsStrings()

	// Get audio devices
	tmpFilter = deviceList
	tmpFilter.FilterPrefix("snd")
	soundDevices := []string{}
	if len(tmpFilter.AsStrings()) > 0 {
		soundDevices = append(soundDevices, "/dev/snd") // Add /dev/snd as a sound device
	}

	return append(videoDevices, soundDevices...)
}

func disconnectSerialFromRPCRouter(ctx context.Context, portAddress string) func() {
	var msgPackRouterAddr = orchestratorConfig.routerSocketPath.String()
	c, err := net.Dial("unix", msgPackRouterAddr)
	if err != nil {
		slog.Error("Failed to connect to router", "addr", msgPackRouterAddr, "err", err)
		return func() {}
	}
	conn := msgpackrpc.NewConnection(c, c, nil, nil, nil)
	go conn.Run()

	if _, _, err := conn.SendRequest(ctx, "$/serial/close", []any{portAddress}); err != nil {
		slog.Error("Failed to send $/serial/close request to router", "addr", ":8900", "err", err)
	}

	return func() {
		defer c.Close()
		defer conn.Close()
		if _, _, err := conn.SendRequest(ctx, "$/serial/open", []any{portAddress}); err != nil {
			slog.Error("Failed to send $/serial/open request to router", "addr", ":8900", "err", err)
		}
	}
}

func compileUploadSketch(ctx context.Context, arduinoApp *app.ArduinoApp, w io.Writer) error {
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
	sketchPath := arduinoApp.MainSketchPath.String()
	buildPath := arduinoApp.SketchBuildPath().String()
	sketchResp, err := srv.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath})
	if err != nil {
		return err
	}
	sketch := sketchResp.GetSketch()
	var profile string
	if micro.OnBoard {
		profile = sketch.GetDefaultProfile().GetName()
	}
	initReq := &rpc.InitRequest{
		Instance:   inst,
		SketchPath: sketchPath,
		Profile:    profile,
	}

	if err := srv.Init(
		initReq,
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			var response string
			switch msg := r.GetMessage().(type) {
			case *rpc.InitResponse_InitProgress:
				if progress := msg.InitProgress.GetTaskProgress(); progress != nil {
					response = helpers.ArduinoCLITaskProgressToString(progress)
				}
				if progress := msg.InitProgress.GetDownloadProgress(); progress != nil {
					response = helpers.ArduinoCLIDownloadProgressToString(progress)
				}
			case *rpc.InitResponse_Error:
				response = "Error: " + msg.Error.String()
			case *rpc.InitResponse_Profile:
				response = fmt.Sprintf(
					"Sketch profile configured: FQBN=%q, Port=%q",
					msg.Profile.GetFqbn(),
					msg.Profile.GetPort(),
				)
			}
			if _, err := w.Write([]byte(response + "\n")); err != nil {
				return err
			}

			return nil
		}),
	); err != nil {
		return err
	}

	var fqbn string
	var port *rpc.Port
	if micro.OnBoard {
		fqbn = "arduino:zephyr:unoq"
	} else {
		resp, err := srv.BoardList(ctx, &rpc.BoardListRequest{
			Instance:                      inst,
			Timeout:                       1000,
			Fqbn:                          "",
			SkipCloudApiForBoardDetection: false,
		})
		if err != nil {
			return err
		}

		var name string
		for _, portItem := range resp.Ports {
			for _, boardItem := range portItem.MatchingBoards {
				if !strings.HasPrefix(boardItem.Fqbn, "arduino") {
					continue
				}
				name = boardItem.Name
				fqbn = boardItem.Fqbn
				port = portItem.Port
				break
			}
		}
		if port == nil {
			return fmt.Errorf("no board detected")
		}
		slog.Debug("Auto selected board", "board_name", name, "fqbn", fqbn, "port", port.Address)
	}

	// build the sketch
	server, getCompileResult := commands.CompilerServerToStreams(ctx, w, w, nil)

	// TODO: add build cache
	if profile == "" {
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
	} else {
		err = srv.Compile(&rpc.CompileRequest{
			Instance:   inst,
			Fqbn:       fqbn,
			SketchPath: sketchPath,
			BuildPath:  buildPath,
		}, server)
		if err != nil {
			return err
		}
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

	if port != nil {
		reconnect := disconnectSerialFromRPCRouter(ctx, port.Address)
		defer reconnect()
	}

	stream, _ := commands.UploadToServerStreams(ctx, w, w)
	return srv.Upload(&rpc.UploadRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: sketchPath,
		Port:       port,
		ImportDir:  buildPath,
	}, stream)
}
