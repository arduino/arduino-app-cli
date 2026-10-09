// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	yaml "github.com/goccy/go-yaml"
	"github.com/gosimple/slug"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/pkg/release"
)

// A release is an app frozen with all its dependencies: <name>-<date>-<target>/ holds
// release.yaml, src/ as authored, prebuild/, the .cache on install, and data/.

const releaseSrcDir = "src"

type BuildReleaseRequest struct {
	// Target defaults to the board running the build.
	Target string
	// ReleaseLabel is an optional label the user attaches to the release. It is
	// stored in the manifest as it is given, and it is absent when left empty.
	ReleaseLabel string
	// Notes is the release note, markdown, and goes in the manifest as it is given.
	Notes string
	// ReleaseName names the archive and the folder it extracts to. Defaults to the app
	// name, the build date and the target board.
	ReleaseName string
	// IncludeData ships the data folder of the app, at the root of the archive.
	IncludeData bool
	// Verbose streams the sketch compile output, as a start does.
	Verbose bool
}

type BuildReleaseResult struct {
	Name     string `json:"name"`
	Target   string `json:"target"`
	FileName string `json:"-"`
}

// ReleaseManifest, ReleaseBrick, and ReleaseModel remain aliases for existing callers.
type ReleaseManifest = release.ReleaseManifest
type ReleaseBrick = release.ReleaseBrick
type ReleaseModel = release.ReleaseModel

// BuildRelease provisions an app for the target board and builds its python
// environment into a release archive, without touching the app folder.
func BuildRelease(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	appToBuild app.ArduinoApp,
	req BuildReleaseRequest,
	cfg config.Configuration,
	cb func(StreamMessage),
) (BuildReleaseResult, io.ReadCloser, error) {
	if cb == nil {
		cb = func(StreamMessage) {}
	}

	// Loaded per build, never by the caller: an index must be the target's own.
	plat, bricksIndex, servicesIndex, modelsIndex, err := targetIndexes(cfg, docker, req.Target)
	if err != nil {
		return BuildReleaseResult{}, nil, err
	}

	bricksIndex = bricksIndex.WithAppBricks(appToBuild.LocalBricks)
	if err := checkBricks(ctx, appToBuild.Descriptor.Bricks, bricksIndex, modelsIndex); err != nil {
		return BuildReleaseResult{}, nil, err
	}

	if err := checkPortCollisions(appToBuild.Descriptor, bricksIndex, servicesIndex); err != nil {
		return BuildReleaseResult{}, nil, err
	}

	// TODO: fail when a service a brick requires is missing for the target. The
	// generator skips it, so it would be missing from the archive for good.

	// The same instant dates the release and names it: the date keeps the releases of
	// an app ordered, as long as there is no version.
	now := time.Now().UTC()
	name := slug.Make(appToBuild.Name)
	if name == "" {
		return BuildReleaseResult{}, nil, fmt.Errorf("%w: the app has no name", ErrBadRequest)
	}
	releaseName := fmt.Sprintf("%s-%s-%s", name, now.Format("20060102-150405"), plat.BoardName)
	// A caller names the archive to follow its output.
	if req.ReleaseName != "" {
		releaseName = req.ReleaseName
	}

	// Staged outside of the app: a release must not inherit its .cache.
	stagingDir, err := cfg.MkTempBuildDir()
	if err != nil {
		return BuildReleaseResult{}, nil, fmt.Errorf("failed to create the staging dir: %w", err)
	}
	// The staging dir holds the archive the stream reads, so its cleanup moves to the
	// stream once the archiving owns it, and runs here on every path that returns first.
	cleanup := func() {
		if err := stagingDir.RemoveAll(); err != nil {
			slog.Warn("cannot remove the release staging dir", slog.String("path", stagingDir.String()), slog.String("error", err.Error()))
		}
	}
	streamOwns := false
	defer func() {
		if !streamOwns {
			cleanup()
		}
	}()

	releaseDir := stagingDir.Join(releaseName)
	srcDir := releaseDir.Join(releaseSrcDir)
	prebuildDir := releaseDir.Join(app.PrebuildDirName)

	cb(StreamMessage{progress: &Progress{Name: "copying the app", Progress: 0.0}})
	if err := stageReleaseSrc(appToBuild, srcDir, bricksIndex); err != nil {
		return BuildReleaseResult{}, nil, err
	}

	// Data is state and not source, so it ships beside src and not in it.
	if dataDir := appToBuild.FullPath.Join("data"); req.IncludeData && dataDir.IsDir() {
		if err := dataDir.CopyDirTo(releaseDir.Join("data")); err != nil {
			return BuildReleaseResult{}, nil, fmt.Errorf("failed to copy the data folder: %w", err)
		}
	}

	manifest := ReleaseManifest{
		Schema:       app.ReleaseManifestSchema,
		Name:         appToBuild.Name,
		ReleaseLabel: req.ReleaseLabel,
		Icon:         appToBuild.Descriptor.Icon,
		Target:       plat.BoardName,
		CreatedAt:    now,
		Notes:        req.Notes,
		Bricks:       releaseBricks(appToBuild.Descriptor),
		Models:       releaseModels(appToBuild.Descriptor, bricksIndex),
		Libraries:    releaseLibraries(ctx, appToBuild),
	}
	if err := writeReleaseManifest(releaseDir, manifest); err != nil {
		return BuildReleaseResult{}, nil, err
	}

	// Loaded back from the staging dir: the provisioning must read the copy that ships.
	stagedApp, err := app.Load(srcDir)
	if err != nil {
		return BuildReleaseResult{}, nil, fmt.Errorf("the staged app is not valid: %w", err)
	}

	// Before the environment, as in a start: it is quick and it is where a build
	// fails, so it does not make anyone wait for the venv to find it out.
	cb(StreamMessage{data: "freezing the compose files", progress: &Progress{Name: "compose files", Progress: 10.0}})
	appEnv := appEnvironment(ctx, stagedApp, bricksIndex, modelsIndex, plat)
	if err := provisioner.Resolve(&stagedApp, prebuildDir, bricksIndex, servicesIndex, cfg, appEnv, plat); err != nil {
		return BuildReleaseResult{}, nil, fmt.Errorf("failed to freeze the compose files: %w", err)
	}

	if err := stageReleaseIndexes(ctx, prebuildDir, appToBuild, bricksIndex, modelsIndex, cfg, appEnv); err != nil {
		return BuildReleaseResult{}, nil, fmt.Errorf("failed to freeze the brick and model indexes: %w", err)
	}

	cb(StreamMessage{data: "building the python environment", progress: &Progress{Name: "python environment", Progress: 20.0}})
	if err := buildPythonEnv(ctx, docker, cfg.PythonImage, srcDir, prebuildDir, cb); err != nil {
		return BuildReleaseResult{}, nil, err
	}

	// The sketch is optional, as it is for the release manifest: an app made of
	// python only ships without a firmware.
	if _, hasSketch := stagedApp.GetSketchPath(); hasSketch {
		cb(StreamMessage{data: "building sketch", progress: &Progress{Name: "sketch", Progress: 80.0}})
		// The compile reads the staged sources and caches in the app folder, as a
		// start does. Only the firmware lands in the release, in prebuild.
		if err := buildSketch(ctx, stagedApp, plat, appToBuild.SketchBuildPath(), prebuildDir, req.Verbose, cb); err != nil {
			return BuildReleaseResult{}, nil, err
		}
	}

	// The archiving streams straight to the caller, so the build never stages the archive
	// to a file of its own: the staging dir goes with the stream it feeds.
	fileName := releaseName + release.ReleaseArchiveExt
	cb(StreamMessage{data: "writing " + fileName, progress: &Progress{Name: "archive", Progress: 90.0}})
	stream := newArchiveStream(releaseDir, cleanup, cb)
	streamOwns = true
	return BuildReleaseResult{
		Name:     name,
		Target:   plat.BoardName,
		FileName: fileName,
	}, stream, nil
}

// writeReleaseManifest writes the file the install keeps as it is: it is the manifest
// of the archive and the marker of the app installed from it.
func writeReleaseManifest(releaseDir *paths.Path, manifest ReleaseManifest) error {
	// The note is markdown and is read by people as well: a block keeps its line breaks
	// where an escaped string would bury them.
	data, err := yaml.MarshalWithOptions(manifest, yaml.UseLiteralStyleIfMultiline(true))
	if err != nil {
		return fmt.Errorf("failed to write the release manifest: %w", err)
	}
	if err := releaseDir.Join(app.ReleaseManifestFileName).WriteFile(data); err != nil {
		return fmt.Errorf("failed to write the release manifest: %w", err)
	}
	return nil
}

// stageReleaseIndexes writes the bricks and the models the app uses where their indexes
// are read from.
func stageReleaseIndexes(
	ctx context.Context,
	prebuildDir *paths.Path,
	appToBuild app.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
	modelsIndex *modelsindex.ModelsIndex,
	cfg config.Configuration,
	appEnv types.Mapping,
) error {
	// A brick the app brings along ships in src, so the board reads that one.
	bricks := make([]bricksindex.Brick, 0, len(appToBuild.Descriptor.Bricks))
	for _, brick := range appToBuild.Descriptor.Bricks {
		if slices.ContainsFunc(appToBuild.LocalBricks, func(local bricksindex.Brick) bool { return local.ID == brick.ID }) {
			continue
		}
		definition, found := bricksIndex.FindBrickByID(brick.ID)
		if !found {
			return fmt.Errorf("brick %q is not in the index", brick.ID)
		}
		bricks = append(bricks, *definition)
	}
	if err := bricksindex.WriteBricksList(prebuildDir, bricks); err != nil {
		return err
	}

	// The models the app is wired with and the handlers they name. A built-in model is
	// included even if it ships with the board image, to avoid "unknown model" errors during the prepare.
	lookup := modelsIndex.NewLookup()
	var models []modelsindex.AIModel
	var handlers []string
	for _, brick := range appToBuild.Descriptor.Bricks {
		definition, found := bricksIndex.FindBrickByID(brick.ID)
		if !found {
			return fmt.Errorf("brick %q is not in the index", brick.ID)
		}
		modelID := selectedModelID(brick, definition)
		if modelID == "" || slices.ContainsFunc(models, func(m modelsindex.AIModel) bool { return m.ID == modelID }) {
			continue
		}
		model, err := lookup.ByID(ctx, modelID)
		if err != nil {
			return err
		}
		if model == nil {
			return fmt.Errorf("model %q is not in the index", modelID)
		}
		models = append(models, *model)
		if model.Deployment != nil && !slices.Contains(handlers, model.Deployment.Handler) {
			handlers = append(handlers, model.Deployment.Handler)
		}
	}
	if err := modelsindex.WriteModelsList(prebuildDir, models); err != nil {
		return err
	}
	// The index reads a missing handlers file as none, unlike the two lists above.
	if len(handlers) == 0 {
		return nil
	}

	// Frozen as the compose files are: what no one answers here stays a reference.
	resolve := frozenLookup(cfg, appEnv)
	return modelsindex.WriteHandlers(cfg.AssetDir(), prebuildDir, handlers, func(data []byte) ([]byte, error) {
		return frozenYAML(data, func(name string) (string, bool) {
			if value, answered := resolve(name); answered {
				return value, true
			}
			return "${" + name + "}", true
		})
	})
}

// releaseBricks is the bricks of the app and the model each is wired with.
func releaseBricks(descriptor app.AppDescriptor) []string {
	return f.Map(descriptor.Bricks, func(brick app.Brick) string {
		return brick.ID
	})
}

// releaseModels is the AI models the bricks of the app are wired with, each stated once.
func releaseModels(descriptor app.AppDescriptor, bricksIndex *bricksindex.BricksIndex) []string {
	models := make([]string, 0, len(descriptor.Bricks))
	for _, brick := range descriptor.Bricks {
		definition, found := bricksIndex.FindBrickByID(brick.ID)
		if !found {
			continue
		}
		modelID := selectedModelID(brick, definition)
		if modelID == "" || slices.Contains(models, modelID) {
			continue
		}
		models = append(models, modelID)
	}
	return models
}

// releaseLibraries is the sketch libraries the app is built with, as name@version. An
// app without a sketch has none, and a listing that fails leaves the manifest without them.
func releaseLibraries(ctx context.Context, arduinoApp app.ArduinoApp) []string {
	if _, hasSketch := arduinoApp.GetSketchPath(); !hasSketch {
		return nil
	}
	libraries, err := ListSketchLibraries(ctx, arduinoApp)
	if err != nil {
		slog.Warn("cannot list the sketch libraries for the release manifest", slog.String("error", err.Error()))
		return nil
	}
	return f.Map(libraries, LibraryReleaseID.String)
}

// targetIndexes are the indexes of the board the release is built for: which bricks and
// services exist, and which compose variant they use, depend on it.
func targetIndexes(cfg config.Configuration, docker command.Cli, target string) (platform.Platform, *bricksindex.BricksIndex, *servicesindex.ServicesIndex, *modelsindex.ModelsIndex, error) {
	plat := platform.GetPlatform(cfg.DataDir())
	if target != "" {
		targetPlatform, ok := platform.ForBoard(target)
		if !ok {
			return platform.Platform{}, nil, nil, nil, fmt.Errorf("%w: unknown target board %q: expected one of %s", ErrBadRequest, target, strings.Join(platform.SupportedBoards(), ", "))
		}
		plat = targetPlatform
	}

	bricksIndex, err := bricksindex.Load(plat, cfg.AssetDir())
	if err != nil {
		return platform.Platform{}, nil, nil, nil, fmt.Errorf("failed to load the bricks index of %s: %w", plat.BoardName, err)
	}
	servicesIndex, err := servicesindex.Load(plat, cfg.AssetDir().Join("services"))
	if err != nil {
		return platform.Platform{}, nil, nil, nil, fmt.Errorf("failed to load the services index of %s: %w", plat.BoardName, err)
	}
	modelsIndex, err := modelsindex.Load(plat, cfg.AssetDir(), cfg.ModelsDir(), cfg.CustomModelsDir(), docker.Client(), cfg)
	if err != nil {
		return platform.Platform{}, nil, nil, nil, fmt.Errorf("failed to load the models index of %s: %w", plat.BoardName, err)
	}
	return plat, bricksIndex, servicesIndex, modelsIndex, nil
}

// stageReleaseSrc copies the app folder as authored: .cache is resolved anew by the
// build and data is staged on its own, when it is asked for.
func stageReleaseSrc(appToBuild app.ArduinoApp, srcDir *paths.Path, bricksIndex *bricksindex.BricksIndex) error {
	if err := srcDir.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create the release src dir: %w", err)
	}

	skipFilter := appSourceFilter(false)
	entries, err := appToBuild.FullPath.ReadDirRecursiveFiltered(skipFilter, skipFilter)
	if err != nil {
		return fmt.Errorf("failed to read the app folder: %w", err)
	}
	for _, entry := range entries {
		relPath, err := entry.RelFrom(appToBuild.FullPath)
		if err != nil {
			return err
		}
		dst := srcDir.JoinPath(relPath)

		// Stat and CopyTo follow the link, as the export does: a release carries what
		// the link points at, which may well be outside the app folder.
		info, err := entry.Stat()
		if err != nil {
			if errors.Is(err, syscall.ELOOP) {
				continue
			}
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}

		if info.IsDir() {
			if err := dst.MkdirAll(); err != nil {
				return fmt.Errorf("failed to create %s: %w", relPath, err)
			}
			continue
		}
		if err := dst.Parent().MkdirAll(); err != nil {
			return fmt.Errorf("failed to create %s: %w", relPath.Parent(), err)
		}

		// app.yaml is written back and not copied: it is where a secret is stored, and
		// a release is shipped, so it goes out redacted as an export does.
		if relPath.String() == "app.yaml" { // nolint:goconst
			desc, err := app.ParseDescriptorFile(entry)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", relPath, err)
			}
			redactSecrets(bricksIndex, &desc)
			data, err := yaml.Marshal(desc)
			if err != nil {
				return fmt.Errorf("failed to write %s: %w", relPath, err)
			}
			if err := dst.WriteFile(data); err != nil {
				return fmt.Errorf("failed to write %s: %w", relPath, err)
			}
			continue
		}

		if err := entry.CopyTo(dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", relPath, err)
		}
	}

	return nil
}

// buildPythonEnv builds the python environment in the runner image, as run.sh would do
// at the first start, and leaves it in the prebuild dir.
func buildPythonEnv(ctx context.Context, docker command.Cli, pythonImage string, srcDir *paths.Path, prebuildDir *paths.Path, cb func(StreamMessage)) error {
	if err := prebuildDir.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create the prebuild dir: %w", err)
	}

	// We create, and then delete, a mount point for the cache in src, which will point to the prebuild dir.
	cacheMountPoint := srcDir.Join(".cache")
	if err := cacheMountPoint.Mkdir(); err != nil {
		return fmt.Errorf("failed to create the cache mount point: %w", err)
	}
	defer func() {
		if err := cacheMountPoint.Remove(); err != nil {
			slog.Warn("cannot remove the cache mount point", slog.String("path", cacheMountPoint.String()), slog.String("error", err.Error()))
		}
	}()

	output := NewCallbackWriter(func(line string) {
		cb(StreamMessage{data: line})
	})
	err := dockerhelper.Run(ctx, docker.Client(), dockerhelper.RunOptions{
		Image: pythonImage,
		Cmd:   []string{"prepare"},
		Binds: []string{
			srcDir.String() + ":/app:ro",
			prebuildDir.String() + ":/app/.cache",
		},
		Stdout: output,
		Stderr: output,
	})
	if err != nil {
		return fmt.Errorf("failed to build the python environment: %w", err)
	}
	return nil
}

// ReleaseFirmwareFileName is the compiled sketch a release ships in its prebuild dir.
const ReleaseFirmwareFileName = "sketch.fw"

func buildSketch(ctx context.Context, appToBuild app.ArduinoApp, platform platform.Platform, buildPath, destPath *paths.Path, verbose bool, cb func(StreamMessage)) error {
	output := NewCallbackWriter(func(line string) {
		cb(StreamMessage{data: line})
	})

	sketchPath, ok := appToBuild.GetSketchPath()
	if !ok {
		return fmt.Errorf("no sketch path found in the Arduino app")
	}
	if err := buildPath.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	srv, inst, err := initializeArduinoCli(ctx, sketchPath, output)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	// The option only exists on the platforms that wait for the Linux side.
	menuOptions, err := GetPlatformMenuOptions(ctx, platform)
	if err != nil {
		slog.Warn("failed to get platform menu options", slog.String("error", err.Error()))
	}
	fqbn := platform.FQBN
	if menuOptions.Has(WaitForApp) {
		fqbn += ":" + WaitForApp.String()
	}

	// Compile the sketch
	if err := compileSketch(
		ctx, srv, inst,
		sketchPath, buildPath,
		platform, fqbn,
		verbose, output,
	); err != nil {
		return err
	}

	// Upload to file
	uploadStream, _ := commands.UploadToServerStreams(ctx, output, output)
	if err := srv.Upload(&rpc.UploadRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: sketchPath.String(),
		ImportDir:  buildPath.String(),
		Verbose:    verbose,
		// There is no board to upload to: "default" is the protocol arduino-cli uses for
		// portless uploads, and it selects the upload.tool.default recipe.
		Port:                 &rpc.Port{Protocol: "default"},
		UploadToFirmwareFile: new(destPath.Join(ReleaseFirmwareFileName).String()),
	}, uploadStream); err != nil {
		return fmt.Errorf("failed to create the sketch artifact: %w", err)
	}

	return nil
}

// archiveStream streams a release archive as it is written, so a caller can save it.
type archiveStream struct {
	reader  *io.PipeReader
	done    chan struct{}
	once    sync.Once
	cleanup func()
}

// newArchiveStream archives releaseDir into a pipe and hands back its read end. The
// cleanup runs once the stream is closed and the archiving has stopped, so the staging
// dir is never removed from under a read.
func newArchiveStream(releaseDir *paths.Path, cleanup func(), cb func(StreamMessage)) *archiveStream {
	reader, writer := io.Pipe()
	stream := &archiveStream{reader: reader, done: make(chan struct{}), cleanup: cleanup}

	go func() {
		defer close(stream.done)
		if err := writeReleaseArchiveTo(releaseDir, writer); err != nil {
			// The error reaches the consumer's Read, so a truncated archive is never read
			// as a good one.
			_ = writer.CloseWithError(fmt.Errorf("failed to write the release archive: %w", err))
			return
		}
		// The write is done only once the last block is read, so the progress is complete
		// before the consumer gets the end of the stream.
		cb(StreamMessage{progress: &Progress{Progress: 100.0}})
		_ = writer.Close()
	}()

	return stream
}

func (a *archiveStream) Read(p []byte) (int, error) { return a.reader.Read(p) }

// Close stops the archiving and removes the staging dir, once the writer has stopped so
// the files are never pulled from under a read.
func (a *archiveStream) Close() error {
	err := a.reader.Close()
	<-a.done
	a.once.Do(a.cleanup)
	return err
}

// Symlinks are kept as they are, and so are the modes of the venv of the prebuild: the
// rest is normalized, so that the archive does not carry the umask of the build machine.
func writeReleaseArchiveTo(releaseDir *paths.Path, w io.Writer) (err error) {
	gzipWriter := gzip.NewWriter(w)
	tarWriter := tar.NewWriter(gzipWriter)
	// A close writes the footer of what it wraps, so a dropped error is a truncated
	// archive reported as a good one.
	defer func() {
		for _, closer := range []io.Closer{tarWriter, gzipWriter} {
			if closeErr := closer.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
	}()

	stagingDir := releaseDir.Parent()
	// The files are read through the staging dir, so that a symlink cannot make the
	// archive pick up something from outside of it.
	stagingRoot, err := os.OpenRoot(stagingDir.String())
	if err != nil {
		return fmt.Errorf("failed to open the staging dir: %w", err)
	}
	defer stagingRoot.Close()

	err = func() error {
		// The same rule the staging used, except that the venv is shipped whole: a
		// package of it may well have a data folder.
		keep := appSourceFilter(true)
		// A symlink is archived as it is, so it is never walked into.
		notSymlink := func(p *paths.Path) bool {
			info, err := p.Lstat()
			return err == nil && info.Mode()&os.ModeSymlink == 0
		}

		entries, err := releaseDir.ReadDirRecursiveFiltered(paths.AndFilter(keep, notSymlink), keep)
		if err != nil {
			return err
		}

		// Write metadata on the top, so we can read these files without full decompression.
		priority := paths.PathList{
			releaseDir.Join(app.ReleaseManifestFileName),
			releaseDir.Join(release.BricksListFileName),
			releaseDir.Join(release.ModelsListFileName),
		}
		entries = slices.DeleteFunc(entries, func(p *paths.Path) bool {
			return slices.ContainsFunc(priority, p.EqualsTo)
		})

		for _, entry := range append(append(paths.PathList{releaseDir}, priority...), entries...) {
			info, err := entry.Lstat()
			if err != nil {
				return err
			}
			relPath, err := entry.RelFrom(stagingDir)
			if err != nil {
				return err
			}

			var linkTarget string
			if info.Mode()&os.ModeSymlink != 0 {
				if linkTarget, err = os.Readlink(entry.String()); err != nil {
					return err
				}
			}

			header, err := tar.FileInfoHeader(info, linkTarget)
			if err != nil {
				return err
			}
			// A tar name is always slash separated.
			header.Name = filepath.ToSlash(relPath.String())
			// The install decides who owns the files.
			header.Uid, header.Gid = 0, 0
			header.Uname, header.Gname = "", ""
			// The modes of the build machine are not shipped, or an app folder left group
			// writable installs group writable. The venv is the exception: it needs its x.
			if _, entry, _ := strings.Cut(header.Name, "/"); !strings.HasPrefix(entry, "prebuild/") {
				switch {
				case info.IsDir():
					header.Mode = 0755
				case info.Mode().IsRegular():
					header.Mode = 0644
				}
			}

			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				continue
			}

			content, err := stagingRoot.Open(relPath.String())
			if err != nil {
				return err
			}
			_, err = io.Copy(tarWriter, content)
			content.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}()
	return err
}
