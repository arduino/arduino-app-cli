// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"github.com/sirupsen/logrus"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// ReleaseFormatVersion is the version of the Arduino App Release format produced by
// `release build`. Bump it on breaking changes to the release layout/manifest.
const ReleaseFormatVersion = "1.0"

// releaseFormatMajor returns the major component of a "major.minor" release format version
// (the part before the first "."). Releases with a different major are considered
// incompatible; a differing minor is assumed backward-compatible.
func releaseFormatMajor(v string) string {
	if i := strings.Index(v, "."); i >= 0 {
		return v[:i]
	}
	return v
}

// ReleaseManifestFileName is the manifest file embedded at the root of an app release
// (and dropped into the app folder on install). Its presence marks an installed app
// as release-managed.
const ReleaseManifestFileName = "arduino-app-release.yaml"

// Limits that mitigate decompression bombs and resource-exhaustion from a crafted release.
const (
	maxReleaseFileSize  = 500 * 1024 * 1024        // per-file uncompressed cap
	maxReleaseTotalSize = 4 * 1024 * 1024 * 1024   // aggregate uncompressed cap across all files
	maxReleaseEntries   = 500 * 1000               // max number of archive entries
	maxManifestSize     = 4 * 1024 * 1024          // the manifest is a few KB; cap generously
)

// minScrubbableSecretLen is the shortest secret value we will blanket-replace by value in the
// frozen compose. Shorter values risk over-matching structural tokens (ports, tags) and are
// covered by the precise env-key scrubbing instead.
const minScrubbableSecretLen = 6

// appHomePlaceholder and hostIPPlaceholder are the tokens substituted into the frozen
// compose file in place of host-specific absolute paths/addresses, so the release can
// be moved between machines. They are re-expanded at `release start` time via the
// process environment.
const (
	appHomePlaceholder = "${APP_HOME}"
	hostIPPlaceholder  = "${HOST_IP}"
)

// ReleaseManifest describes the provenance and compatibility of an Arduino App Release.
type ReleaseManifest struct {
	FormatVersion    string          `yaml:"format_version"`
	AppName          string          `yaml:"app_name"`
	Release          string          `yaml:"release"`
	CreatedAt        time.Time       `yaml:"created_at"`
	SourceCLIVersion string          `yaml:"source_cli_version"`
	RunnerVersion    string          `yaml:"runner_version"`
	PythonImage      string          `yaml:"python_image"`
	BoardName        string          `yaml:"board_name"`
	FQBN             string          `yaml:"fqbn"`
	Arch             string          `yaml:"arch"`
	Images           []string        `yaml:"images,omitempty"`
	Models           []ReleaseModel  `yaml:"models,omitempty"`
	RequiredSecrets  []ReleaseSecret `yaml:"required_secrets,omitempty"`
}

// ReleaseSecret declares a secret brick variable whose value is NOT bundled in the release.
// The value must be supplied on the destination (in the app's data/secrets.env) before the
// app can start.
type ReleaseSecret struct {
	Name  string `yaml:"name"`
	Brick string `yaml:"brick,omitempty"`
}

// ReleaseModel records an AI model required by the app and how it is provided by the
// release.
type ReleaseModel struct {
	ID        string   `yaml:"id"`
	Name      string   `yaml:"name,omitempty"`
	Runner    string   `yaml:"runner,omitempty"`
	PreLoaded bool     `yaml:"preloaded"`       // shipped inside a (pinned) container image
	Bundled   bool     `yaml:"bundled"`         // model files are bundled in this release
	Paths     []string `yaml:"paths,omitempty"` // when bundled: artifact paths relative to the custom-models dir
}

// releaseModelsDirPrefix is the archive sub-path under which bundled model artifacts are
// stored (mirroring their layout under the custom-models dir).
const releaseModelsDirPrefix = "models"

// customModelDirPlaceholder is the token substituted for the host custom-models dir in the
// frozen compose, re-expanded on the destination at `release start`.
const customModelDirPlaceholder = "${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}"

// tarPath describes an additional file or directory tree to include in a release archive
// at the given archive-relative path (under the release root).
type tarPath struct {
	archiveRel string
	source     *paths.Path
}

// ErrNotFrozenRelease is returned when an operation expects an app installed from a release
// but the app is missing the release artifacts (manifest / frozen compose).
var ErrNotFrozenRelease = errors.New("app is not a release-installed app")

// ErrIncompatibleRelease is returned by InstallRelease when the release targets a
// different board than the current one and installation was not forced.
var ErrIncompatibleRelease = errors.New("incompatible release")

// BuildRelease builds a self-contained, reproducible Arduino App Release (.tar.gz)
// from an app that has already been built and started at least once.
//
// The release contains the user code, the prebuilt sketch binary (.cache/sketch), the
// provisioned Python venv (.cache/...), a flattened+version-pinned compose file and a
// manifest. It deliberately requires the app to be pre-built: it never compiles the
// sketch nor provisions the venv itself.
func BuildRelease(
	ctx context.Context,
	bricksIndex *bricksindex.BricksIndex,
	modelsIndex *modelsindex.ModelsIndex,
	arduinoApp app.ArduinoApp,
	cfg config.Configuration,
	plat platform.Platform,
	cliVersion string,
	releaseNumber string,
	outputPath *paths.Path,
	includeModels bool,
	keepSecrets bool,
	cb func(StreamMessage),
) error {
	bricksIndex = bricksIndex.WithAppBricks(arduinoApp.LocalBricks)

	emit := func(msg string) {
		if cb != nil {
			cb(StreamMessage{data: msg})
		}
	}

	// A release number identifies this build of the release. Default to a timestamp.
	if releaseNumber == "" {
		releaseNumber = time.Now().Format("20060102150405")
	}
	emit(fmt.Sprintf("Release number: %s", releaseNumber))

	// 1. Validate the app is pre-built.
	if err := validatePrebuilt(arduinoApp); err != nil {
		return err
	}
	emit("Pre-built artifacts found")

	// 2. Handle secret brick variables. By default they are scrubbed from the release and
	// replaced with ${NAME} env placeholders, to be provided on the destination; --keep-secrets
	// keeps the values in the release (making the archive sensitive).
	descriptor, err := app.ParseDescriptorFile(arduinoApp.GetDescriptorPath())
	if err != nil {
		return fmt.Errorf("failed to read app.yaml: %w", err)
	}
	var requiredSecrets []ReleaseSecret
	var secretValues map[string][]string
	if keepSecrets {
		emit("Warning: --keep-secrets keeps configured secrets (API keys, passwords) in the release. Treat the archive as sensitive.")
	} else {
		requiredSecrets, secretValues = scrubAppSecrets(&descriptor, bricksIndex)
		if len(requiredSecrets) > 0 {
			emit(fmt.Sprintf("Scrubbed %d secret(s); they must be provided on the destination in data/secrets.env", len(requiredSecrets)))
		}
	}
	appYamlBytes, err := yaml.Marshal(descriptor)
	if err != nil {
		return fmt.Errorf("failed to marshal app.yaml: %w", err)
	}

	// 3. Resolve the AI models required by the app and decide which to bundle.
	modelEntries, modelArtifacts, err := resolveAppModels(ctx, arduinoApp, modelsIndex, cfg, includeModels, emit)
	if err != nil {
		return err
	}

	// 4. Flatten the generated compose into a single, version-pinned, relocatable file,
	// scrubbing the secret values unless they are being kept.
	emit("Freezing compose with pinned container versions")
	envs := getAppEnvironmentVariables(ctx, arduinoApp, bricksIndex, modelsIndex, plat, cfg)
	frozenCompose, err := buildFrozenCompose(ctx, arduinoApp, cfg, envs, secretValues)
	if err != nil {
		return fmt.Errorf("failed to freeze compose file: %w", err)
	}

	// 5. Build the manifest.
	manifest := ReleaseManifest{
		FormatVersion:    ReleaseFormatVersion,
		AppName:          arduinoApp.Name,
		Release:          releaseNumber,
		CreatedAt:        time.Now().UTC(),
		SourceCLIVersion: cliVersion,
		RunnerVersion:    cfg.RunnerVersion,
		PythonImage:      cfg.PythonImage,
		BoardName:        plat.BoardName,
		FQBN:             plat.FQBN,
		Arch:             runtime.GOARCH,
		Images:           extractComposeImages(frozenCompose),
		Models:           modelEntries,
		RequiredSecrets:  requiredSecrets,
	}
	manifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal release manifest: %w", err)
	}

	// 6. Write the tar.gz. app.yaml is provided here (scrubbed) instead of from the walk.
	emit("Writing release archive")
	rootFolderName := releaseRootFolderName(arduinoApp.Name)
	extraFiles := map[string][]byte{
		"app.yaml":              appYamlBytes,
		ReleaseManifestFileName: manifestBytes,
		".cache/" + arduinoApp.ReleaseComposeFilePath().Base(): frozenCompose,
	}
	if err := writeAppTarGz(arduinoApp.FullPath, rootFolderName, outputPath, extraFiles, modelArtifacts); err != nil {
		return fmt.Errorf("failed to write release archive: %w", err)
	}

	return nil
}

// scrubAppSecrets replaces, in place, the value of every brick variable flagged as secret
// (and currently set to a non-empty value) with a ${NAME} env placeholder. It returns the
// list of secrets that must be provided on the destination, plus a name->values map of the
// scrubbed plaintext values (used to also strip those values from the frozen compose). The
// env var name is the brick variable name (which is also how it reaches the container).
//
// The value map holds a slice per name because two bricks can declare a secret with the same
// name but different values; every distinct value must be stripped, so keying a single value
// per name (and thus dropping the others) would risk leaving one baked in the compose.
func scrubAppSecrets(desc *app.AppDescriptor, bricksIndex *bricksindex.BricksIndex) ([]ReleaseSecret, map[string][]string) {
	var required []ReleaseSecret
	seen := make(map[string]struct{})
	values := make(map[string][]string)
	for i := range desc.Bricks {
		b := &desc.Bricks[i]
		brickDef, found := bricksIndex.FindBrickByID(b.ID)
		if !found {
			continue
		}
		for name, value := range b.Variables {
			if value == "" {
				continue
			}
			vDef, ok := brickDef.GetVariable(name)
			if !ok || !vDef.Secret {
				continue
			}
			if !slices.Contains(values[name], value) {
				values[name] = append(values[name], value)
			}
			b.Variables[name] = "${" + name + "}"
			if _, dup := seen[name]; !dup {
				seen[name] = struct{}{}
				required = append(required, ReleaseSecret{Name: name, Brick: b.ID})
			}
		}
	}
	return required, values
}

// resolveAppModels inspects the models referenced by the app's bricks and returns the
// manifest entries plus the artifact paths to bundle. Preloaded models live inside
// (version-pinned) container images and are not bundled. Every other required model is
// bundled into the release from its on-disk artifacts so that installing the release never
// has to download a model; only containers are pulled (at start).
//
// If a non-preloaded model has no locatable on-disk artifacts, packaging fails (the release
// would otherwise be incomplete) — unless includeModels is false, which intentionally
// excludes models and leaves installing them to the destination.
func resolveAppModels(
	ctx context.Context,
	arduinoApp app.ArduinoApp,
	modelsIndex *modelsindex.ModelsIndex,
	cfg config.Configuration,
	includeModels bool,
	emit func(string),
) ([]ReleaseModel, []tarPath, error) {
	seen := make(map[string]struct{})
	bundledArchiveRel := make(map[string]struct{})
	var entries []ReleaseModel
	var toBundle []tarPath

	customModelsDir := cfg.CustomModelsDir()

	for _, brick := range arduinoApp.Descriptor.Bricks {
		modelID := brick.Model
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}

		model, err := modelsIndex.GetModelByID(ctx, modelID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve model %q required by brick %q: %w", modelID, brick.ID, err)
		}
		if model == nil {
			return nil, nil, fmt.Errorf("%w: model %q required by brick %q is not available", ErrBadRequest, modelID, brick.ID)
		}

		entry := ReleaseModel{ID: model.ID, Name: model.Name, Runner: model.Runner}

		// Preloaded models travel inside their (pinned) container image.
		if model.Deployment != nil && model.Deployment.PreLoaded {
			entry.PreLoaded = true
			entries = append(entries, entry)
			continue
		}

		if !includeModels {
			emit(fmt.Sprintf("Skipping model %q (--no-models); it must be installed on the destination", model.ID))
			entries = append(entries, entry)
			continue
		}

		// Bundle the model's on-disk artifacts so the release is self-contained.
		artifacts := modelsIndex.GetModelLocalArtifacts(*model)
		if len(artifacts) == 0 {
			return nil, nil, fmt.Errorf("%w: model %q (required by brick %q) is not installed or its files could not be located; install it before packaging, or use --no-models",
				ErrBadRequest, model.ID, brick.ID)
		}

		for _, artifact := range artifacts {
			rel, relErr := filepath.Rel(customModelsDir.String(), artifact.String())
			if relErr != nil || strings.HasPrefix(rel, "..") {
				return nil, nil, fmt.Errorf("%w: model %q has an artifact outside the models dir (%s); cannot bundle it",
					ErrBadRequest, model.ID, artifact.String())
			}
			rel = filepath.ToSlash(rel)
			entry.Paths = append(entry.Paths, rel)
			if _, dup := bundledArchiveRel[rel]; dup {
				continue // shared artifact already queued (e.g. models sharing a repository folder)
			}
			bundledArchiveRel[rel] = struct{}{}
			toBundle = append(toBundle, tarPath{
				archiveRel: releaseModelsDirPrefix + "/" + rel,
				source:     artifact,
			})
		}
		entry.Bundled = true
		emit(fmt.Sprintf("Bundling model %q", model.ID))
		entries = append(entries, entry)
	}

	return entries, toBundle, nil
}

// validatePrebuilt ensures the app has the artifacts a release needs: a compiled sketch
// (when the app has a sketch) and a provisioned Python venv, plus a generated compose.
func validatePrebuilt(arduinoApp app.ArduinoApp) error {
	if arduinoApp.AppComposeFilePath().NotExist() {
		return fmt.Errorf("%w: app has not been started yet (missing %s). Start the app once before packaging",
			ErrBadRequest, arduinoApp.AppComposeFilePath().Base())
	}

	if _, ok := arduinoApp.GetSketchPath(); ok {
		buildPath := arduinoApp.SketchBuildPath()
		if buildPath.NotExist() {
			return fmt.Errorf("%w: sketch has not been compiled yet (missing %s). Start the app once before packaging",
				ErrBadRequest, buildPath.String())
		}
		if entries, err := buildPath.ReadDir(); err != nil || len(entries) == 0 {
			return fmt.Errorf("%w: sketch build directory is empty. Start the app once before packaging", ErrBadRequest)
		}
	}

	if !hasProvisionedVenv(arduinoApp.ProvisioningStateDir()) {
		return fmt.Errorf("%w: no provisioned Python venv found under %s. Start the app once before packaging",
			ErrBadRequest, arduinoApp.ProvisioningStateDir().String())
	}

	return nil
}

// hasProvisionedVenv reports whether a Python virtual environment exists somewhere under
// the given .cache directory, detected by the presence of a pyvenv.cfg file. This avoids
// hard-coding the venv directory name, which is owned by the python runner image.
// It uses filepath.WalkDir, which does not follow symlinks, so it never wanders out of the
// app tree via the venv's internal links.
func hasProvisionedVenv(cacheDir *paths.Path) bool {
	if cacheDir.NotExist() {
		return false
	}
	found := false
	_ = filepath.WalkDir(cacheDir.String(), func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignore unreadable entries
		}
		if !d.IsDir() && d.Name() == "pyvenv.cfg" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// Sentinels are substituted for the host-specific env vars before running
// `docker compose config`, so their expanded occurrences in the flattened output can be
// reverse-mapped to portable placeholders without matching real IPs/paths — or substrings
// of them (e.g. host IP 192.168.1.5 inside 192.168.1.50) — elsewhere in the file. They are
// unique, non-colliding tokens; the path-like ones stay absolute so `docker compose config`
// still treats them as bind-mount sources rather than named volumes.
const (
	sentinelHostIP    = "__ARDUINO_RELEASE_PLACEHOLDER_HOST_IP__"
	sentinelAppHome   = "/__ARDUINO_RELEASE_PLACEHOLDER_APP_HOME__"
	sentinelModelsDir = "/__ARDUINO_RELEASE_PLACEHOLDER_CUSTOM_MODEL_DIR__"
)

// buildFrozenCompose flattens the app's generated compose files (merging all includes and
// resolving concrete image references) into a single document, then re-tokenizes the
// host-specific app path, models dir and host IP so the result is relocatable, and finally
// removes any secret material.
func buildFrozenCompose(ctx context.Context, arduinoApp app.ArduinoApp, cfg config.Configuration, envs helpers.EnvVars, secretValues map[string][]string) ([]byte, error) {
	args := []string{"docker", "compose", "-f", arduinoApp.AppComposeFilePath().String()}
	if override := arduinoApp.AppComposeOverrideFilePath(); override.Exist() {
		args = append(args, "-f", override.String())
	}
	args = append(args, "config")

	// Run with the host-specific vars set to unique sentinels, so ${HOST_IP}/${APP_HOME}/
	// ${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR} interpolate to the sentinels (not to real
	// host values) and can be reverse-mapped unambiguously below. Explicitly setting the
	// models dir is required because it is otherwise inherited from the daemon environment.
	frozenEnvs := make(helpers.EnvVars, len(envs)+3)
	for k, v := range envs {
		frozenEnvs[k] = v
	}
	frozenEnvs["HOST_IP"] = sentinelHostIP
	frozenEnvs["APP_HOME"] = sentinelAppHome
	frozenEnvs["ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR"] = sentinelModelsDir

	process, err := paths.NewProcess(frozenEnvs.AsList(), args...)
	if err != nil {
		return nil, err
	}
	stdout, stderr, err := process.RunAndCaptureOutput(ctx)
	if err != nil {
		return nil, fmt.Errorf("docker compose config failed: %w: %s", err, string(stderr))
	}

	flattened := tokenizeHostSpecificValues(string(stdout), cfg.CustomModelsDir().String(), arduinoApp.FullPath.String())

	// The provisioned compose bakes host-specific env values (HOST_IP, MODELS_PATH,
	// CUSTOM_MODEL_PATH, ...) as literals into the generated service `environment:` blocks
	// and as bind-mount source/target paths. The sentinel reverse-mapping above only catches
	// ${VAR} references, so these literals survive and would leak the build machine's IP and
	// absolute paths into the release. Neutralize them to ${KEY} placeholders, which the
	// destination re-provides at run time (getAppEnvironmentVariables) for docker compose to
	// interpolate.
	flattened = tokenizeHostSpecificEnv(flattened, envs)

	// Remove secret material: rewrite the secret env entries by key, then strip any
	// remaining literal secret values (e.g. embedded in a connection string), failing if
	// any survive so the release never carries a secret.
	names := make([]string, 0, len(secretValues))
	for name := range secretValues {
		names = append(names, name)
	}
	flattened = scrubComposeSecretKeys(flattened, names)
	flattened, err = scrubComposeSecretValues(flattened, secretValues)
	if err != nil {
		return nil, err
	}

	return []byte(flattened), nil
}

// tokenizeHostSpecificValues reverse-maps the sentinels injected before `docker compose
// config` back to portable ${...} placeholders. As a safety net it also maps any literally
// baked host paths (custom-models dir, app home) that did not come through the env vars;
// those are long, unique absolute paths, so — unlike an IP — they cannot over-match a
// substring. The host IP is intentionally handled only via its sentinel.
func tokenizeHostSpecificValues(flattened, customModelsDir, appHome string) string {
	flattened = strings.ReplaceAll(flattened, sentinelModelsDir, customModelDirPlaceholder)
	flattened = strings.ReplaceAll(flattened, sentinelAppHome, appHomePlaceholder)
	flattened = strings.ReplaceAll(flattened, sentinelHostIP, hostIPPlaceholder)
	// Guard against empty/root paths, which would make ReplaceAll match everywhere.
	if customModelsDir != "" && customModelsDir != "/" {
		flattened = strings.ReplaceAll(flattened, customModelsDir, customModelDirPlaceholder)
	}
	if appHome != "" && appHome != "/" {
		flattened = strings.ReplaceAll(flattened, appHome, appHomePlaceholder)
	}
	return flattened
}

// hostSpecificEnvKeys lists environment variables whose values are specific to the machine
// that built the release: the host IP and the absolute AI-model directories. Unlike the
// pinned image/config values a frozen release deliberately keeps, these must become ${KEY}
// placeholders so the release is relocatable. The destination re-provides every one of them
// at run time via getAppEnvironmentVariables (MODELS_PATH unconditionally; HOST_IP and
// CUSTOM_MODEL_PATH from the same brick/model sources that populated them at build time), so
// docker compose interpolates them for both `environment:` blocks and bind-mount paths.
var hostSpecificEnvKeys = []string{"HOST_IP", "MODELS_PATH", "CUSTOM_MODEL_PATH"}

// tokenizeHostSpecificEnv rewrites the host-specific env values baked as literals into the
// flattened compose back to ${KEY} placeholders. It does so in two complementary ways:
//   - by key: every `KEY: <value>` / `- KEY=<value>` entry for a host-specific key becomes
//     ${KEY}, regardless of the literal value (this fixes the `environment:` blocks, whose
//     values never came through a ${...} reference the sentinel mapping could catch);
//   - by value: the literal MODELS_PATH / CUSTOM_MODEL_PATH paths are replaced wherever else
//     they appear (e.g. bind-mount source/target). Those are long, unique absolute paths, so
//     — unlike an IP — value replacement cannot over-match a substring; HOST_IP is therefore
//     handled by key only.
func tokenizeHostSpecificEnv(compose string, envs helpers.EnvVars) string {
	compose = scrubComposeSecretKeys(compose, hostSpecificEnvKeys)
	// Replace the longer path first so that when one value is a prefix of the other (e.g.
	// CUSTOM_MODEL_PATH nested under MODELS_PATH) the more specific path is tokenized to its
	// own placeholder instead of being partially rewritten via the shorter one.
	pathKeys := []string{"MODELS_PATH", "CUSTOM_MODEL_PATH"}
	sort.Slice(pathKeys, func(i, j int) bool { return len(envs[pathKeys[i]]) > len(envs[pathKeys[j]]) })
	for _, key := range pathKeys {
		v := envs[key]
		if v == "" || v == "/" {
			continue
		}
		compose = strings.ReplaceAll(compose, v, "${"+key+"}")
	}
	return compose
}

// scrubComposeSecretKeys rewrites the environment entries for the given secret variable
// names so their value becomes a ${NAME} placeholder, resolved from the process environment
// at run time. It targets the environment keys (in both the map and list compose forms)
// rather than the raw secret values, so it never leaves a plaintext value behind.
func scrubComposeSecretKeys(compose string, names []string) string {
	for _, name := range names {
		quoted := regexp.QuoteMeta(name)
		// map form:  "      DB_PASSWORD: <value>"
		reMap := regexp.MustCompile(`(?m)^(\s+)` + quoted + `:[ \t].*$`)
		compose = reMap.ReplaceAllStringFunc(compose, func(line string) string {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			return indent + name + ": ${" + name + "}"
		})
		// list form: "      - DB_PASSWORD=<value>"
		reList := regexp.MustCompile(`(?m)^(\s*)-[ \t]+` + quoted + `=.*$`)
		compose = reList.ReplaceAllStringFunc(compose, func(line string) string {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			return indent + "- " + name + "=${" + name + "}"
		})
	}
	return compose
}

// scrubComposeSecretValues removes any remaining occurrence of a secret's literal value
// from the compose, replacing it with the ${NAME} placeholder. This catches values that
// scrubComposeSecretKeys misses because they are embedded inside a larger value (e.g. a
// password inside a "postgres://user:pass@host" connection string). Longer values are
// replaced first so a shorter secret that is a substring of a longer one cannot corrupt it.
// As defense-in-depth it returns an error if any secret value still appears afterwards, so
// the release is never written with secret material in it.
func scrubComposeSecretValues(compose string, values map[string][]string) (string, error) {
	// Flatten to (name, value) pairs and replace longest values first, so a shorter secret
	// that is a substring of a longer one cannot corrupt the longer replacement.
	type sv struct{ name, value string }
	var pairs []sv
	for name, vs := range values {
		for _, v := range vs {
			if v != "" {
				pairs = append(pairs, sv{name, v})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return len(pairs[i].value) > len(pairs[j].value) })

	for _, p := range pairs {
		// Only blanket-replace values long enough to be distinctive. A very short value
		// (e.g. "1", "8080", "true") would otherwise over-match unrelated tokens like ports
		// or image tags — corrupting the compose, or tripping a false leak-abort on a
		// coincidental structural match. Short secret values are still scrubbed precisely by
		// scrubComposeSecretKeys via their own env entry.
		if len(p.value) < minScrubbableSecretLen {
			continue
		}
		compose = strings.ReplaceAll(compose, p.value, "${"+p.name+"}")
	}

	// Defense-in-depth: after all replacements, verify no (distinctive) secret value remains
	// baked in — abort rather than ship one. Short values are excluded for the reason above.
	leaked := map[string]struct{}{}
	for _, p := range pairs {
		if len(p.value) < minScrubbableSecretLen {
			continue
		}
		if strings.Contains(compose, p.value) {
			leaked[p.name] = struct{}{}
		}
	}
	if len(leaked) > 0 {
		names := make([]string, 0, len(leaked))
		for n := range leaked {
			names = append(names, n)
		}
		sort.Strings(names)
		return "", fmt.Errorf("%w: secret value(s) %s could not be fully removed from the frozen compose; aborting to avoid leaking them (rename/lengthen the value or use --keep-secrets)",
			ErrBadRequest, strings.Join(names, ", "))
	}
	return compose, nil
}

// extractComposeImages returns the list of container images referenced by a compose file.
func extractComposeImages(composeBytes []byte) []string {
	var doc struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(composeBytes, &doc); err != nil {
		slog.Warn("failed to parse frozen compose for image list", slog.Any("error", err))
		return nil
	}
	seen := make(map[string]struct{})
	var images []string
	for _, svc := range doc.Services {
		if svc.Image == "" {
			continue
		}
		if _, ok := seen[svc.Image]; ok {
			continue
		}
		seen[svc.Image] = struct{}{}
		images = append(images, svc.Image)
	}
	return images
}

// releaseRootFolderName returns a filesystem-safe root folder name for the release
// archive, mirroring the behavior of the zip export.
func releaseRootFolderName(appName string) string {
	name := strings.ToLower(strings.ReplaceAll(appName, " ", "-"))
	if name == "" {
		name = "app-release"
	}
	return name
}

// writeAppTarGz writes the app folder (rooted at rootFolderName) as a gzip-compressed
// tarball at outputPath. It preserves symlinks and file modes (required for the venv),
// excludes the data folder and the non-portable generated compose files, injects the
// provided extraFiles (manifest + frozen compose) and bundles any extraPaths (AI model
// files or folders) under the same root.
func writeAppTarGz(sourcePath *paths.Path, rootFolderName string, outputPath *paths.Path, extraFiles map[string][]byte, extraPaths []tarPath) error {
	out, err := os.Create(outputPath.String())
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// shouldSkip decides, by app-relative slash path, whether to exclude an app entry.
	shouldSkip := func(rel string) bool {
		switch rel {
		case "data": // user/runtime data must not be shipped
			return true
		case "app.yaml": // provided via extraFiles (secrets scrubbed)
			return true
		case ".cache/app-compose.yaml", ".cache/app-compose-overrides.yaml": // host-path baked; superseded by release-compose.yaml
			return true
		case ReleaseManifestFileName: // regenerated below
			return true
		}
		return false
	}

	// Bundle the app folder under rootFolderName.
	if err := addTreeToTar(tw, sourcePath.String(), rootFolderName, shouldSkip); err != nil {
		return err
	}

	// Bundle the extra files/trees (e.g. AI model artifacts).
	for _, p := range extraPaths {
		base := filepath.ToSlash(filepath.Join(rootFolderName, p.archiveRel))
		if err := addPathToTar(tw, p.source.String(), base); err != nil {
			return fmt.Errorf("bundling %s: %w", p.archiveRel, err)
		}
	}

	// Inject the synthetic files (manifest + frozen compose).
	for relName, content := range extraFiles {
		header := &tar.Header{
			Name:    filepath.ToSlash(filepath.Join(rootFolderName, relName)),
			Mode:    0o644,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(content); err != nil {
			return err
		}
	}

	return nil
}

// addPathToTar bundles a single source path (file, directory tree or symlink) into tw at
// archiveName. Directories are recursed via addTreeToTar.
func addPathToTar(tw *tar.Writer, source string, archiveName string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return addTreeToTar(tw, source, archiveName, nil)
	}

	var linkTarget string
	if info.Mode()&os.ModeSymlink != 0 {
		if linkTarget, err = os.Readlink(source); err != nil {
			return err
		}
	}
	header, err := tar.FileInfoHeader(info, linkTarget)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(archiveName)
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		file, err := os.Open(source)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := io.Copy(tw, file); err != nil {
			return err
		}
	}
	return nil
}

// addTreeToTar walks the directory tree at sourceRoot and writes its entries into tw under
// archiveBase. WalkDir does not follow symlinks, so they are archived as links (preserving
// the venv's internal structure). The optional skip predicate (keyed by source-relative
// slash path) excludes entries; returning true on a directory prunes it.
func addTreeToTar(tw *tar.Writer, sourceRoot string, archiveBase string, skip func(rel string) bool) error {
	return filepath.WalkDir(sourceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == sourceRoot {
			return nil // the base itself is implied by its entries
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skip != nil && skip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Lstat so symlinks are described as links, not their targets.
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}

		var linkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			if linkTarget, err = os.Readlink(path); err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join(archiveBase, rel))
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, file)
			file.Close()
			if copyErr != nil {
				return copyErr
			}
		}
		return nil
	})
}

// InstallRelease extracts an Arduino App Release into the apps directory so it is ready to
// be launched with `release start`. It validates board compatibility against the current
// platform (unless force is set) and returns the new app ID together with the manifest.
func InstallRelease(
	ctx context.Context,
	cfg config.Configuration,
	tarPath *paths.Path,
	idProvider *app.IDProvider,
	bricksIndex *bricksindex.BricksIndex,
	modelsIndex *modelsindex.ModelsIndex,
	plat platform.Platform,
	force bool,
) (app.ID, *ReleaseManifest, error) {
	if tarPath == nil {
		return app.ID{}, nil, fmt.Errorf("internal error: tarPath cannot be nil")
	}

	rootPrefix, manifest, err := readReleaseMetadata(tarPath)
	if err != nil {
		return app.ID{}, nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	// Format-version gate: a release written by a future CLI with an incompatible layout must
	// not install silently and fail obscurely later. Compare the major version.
	if manifest.FormatVersion != "" && releaseFormatMajor(manifest.FormatVersion) != releaseFormatMajor(ReleaseFormatVersion) {
		return app.ID{}, manifest, fmt.Errorf("%w: release format version %q is not supported by this CLI (expected %q)",
			ErrIncompatibleRelease, manifest.FormatVersion, ReleaseFormatVersion)
	}

	// Compatibility check: the prebuilt sketch binary and images are board/arch specific.
	if !force {
		if manifest.BoardName != "" && plat.BoardName != "" && manifest.BoardName != plat.BoardName {
			return app.ID{}, manifest, fmt.Errorf("%w: release was built for board %q but this board is %q (use --force to override)",
				ErrIncompatibleRelease, manifest.BoardName, plat.BoardName)
		}
		if manifest.Arch != "" && manifest.Arch != runtime.GOARCH {
			return app.ID{}, manifest, fmt.Errorf("%w: release was built for architecture %q but this host is %q (use --force to override)",
				ErrIncompatibleRelease, manifest.Arch, runtime.GOARCH)
		}
	}

	rawAppName := rootPrefix
	if rawAppName == "" {
		rawAppName = strings.TrimSuffix(tarPath.Base(), ".tar.gz")
		rawAppName = strings.TrimSuffix(rawAppName, filepath.Ext(rawAppName))
	}

	finalDestPath, appExists := findAppPathByName(rawAppName, cfg)
	if appExists {
		suffix := time.Now().Format("-20060102-150405")
		finalDestPath, _ = findAppPathByName(rawAppName+suffix, cfg)
	}

	if !app.IsValidFolderName(finalDestPath.Base()) {
		return app.ID{}, manifest, fmt.Errorf("%w: root folder name %q is not valid", ErrBadRequest, finalDestPath.Base())
	}

	tempDestDir, err := paths.MkTempDir(finalDestPath.Parent().String(), "tmp_pkg_")
	if err != nil {
		return app.ID{}, manifest, fmt.Errorf("unable to create temp app directory: %w", err)
	}
	defer func() { _ = tempDestDir.RemoveAll() }()

	if err := extractTarGz(tarPath, tempDestDir, rootPrefix); err != nil {
		return app.ID{}, manifest, err
	}

	if finalDestPath.Exist() {
		return app.ID{}, manifest, ErrAppAlreadyExists
	}

	// Restore bundled AI models into the custom-models dir (outside the app folder), then
	// drop the staging "models" folder so it is not moved into the apps directory.
	if err := restoreBundledModels(cfg, manifest, tempDestDir, force); err != nil {
		return app.ID{}, manifest, err
	}
	_ = tempDestDir.Join(releaseModelsDirPrefix).RemoveAll()

	// Turn the extracted release into a regular (but release-flagged) app: localize the
	// frozen compose to this machine and stamp the release metadata into app.yaml. From now
	// on the normal `app start`/`app stop` commands operate on it.
	if err := localizeInstalledRelease(ctx, cfg, bricksIndex, modelsIndex, plat, manifest, tempDestDir, finalDestPath); err != nil {
		return app.ID{}, manifest, err
	}

	// Drop a template data/secrets.env for the user to fill in (only if the release declares
	// required secrets and the file does not already exist).
	if err := writeSecretsTemplate(manifest, tempDestDir); err != nil {
		return app.ID{}, manifest, err
	}

	// Validate the extracted app before moving it to its final destination.
	if _, err := app.Load(tempDestDir); err != nil {
		return app.ID{}, manifest, fmt.Errorf("%w: invalid app: %v", ErrBadRequest, err)
	}

	if err := tempDestDir.Rename(finalDestPath); err != nil {
		return app.ID{}, manifest, fmt.Errorf("failed to finalize release install: %w", err)
	}

	id, err := idProvider.IDFromPath(finalDestPath)
	if err != nil {
		return app.ID{}, manifest, err
	}

	return id, manifest, nil
}

// restoreBundledModels copies the model artifacts bundled in the release into the
// destination custom-models dir, preserving their relative layout so the frozen compose's
// ${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR} placeholders resolve correctly. This only copies
// from the archive — it never downloads. Artifacts already present are left untouched unless
// force is set.
func restoreBundledModels(cfg config.Configuration, manifest *ReleaseManifest, extractedAppDir *paths.Path, force bool) error {
	if manifest == nil {
		return nil
	}
	modelsRoot := extractedAppDir.Join(releaseModelsDirPrefix)
	for _, m := range manifest.Models {
		if !m.Bundled {
			continue
		}
		for _, rel := range m.Paths {
			if rel == "" {
				continue
			}
			// The manifest is attacker-controlled; a "../" or absolute path would let the
			// copy read from / write to outside the extracted tree and the custom-models dir.
			// Mirror the validation resolveAppModels does at build time.
			clean := filepath.ToSlash(filepath.Clean(rel))
			if filepath.IsAbs(rel) || clean == ".." || strings.HasPrefix(clean, "../") {
				return fmt.Errorf("%w: model %q declares an illegal artifact path %q", ErrBadRequest, m.ID, rel)
			}
			src := modelsRoot.Join(filepath.FromSlash(clean))
			if src.NotExist() {
				return fmt.Errorf("%w: model %q artifact %q declared in the manifest is missing from the release archive",
					ErrBadRequest, m.ID, rel)
			}
			dst := cfg.CustomModelsDir().Join(filepath.FromSlash(clean))
			if err := mergeCopy(src, dst, force); err != nil {
				return fmt.Errorf("installing model %q: %w", m.ID, err)
			}
		}
	}
	return nil
}

// mergeCopy copies src (a file or directory) to dst, creating parent directories. For
// directories it merges into an existing destination. Existing files are kept unless force
// is set. It never follows or copies into locations outside dst.
func mergeCopy(src, dst *paths.Path, force bool) error {
	info, err := os.Lstat(src.String())
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := dst.MkdirAll(); err != nil {
			return err
		}
		entries, err := src.ReadDir()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := mergeCopy(entry, dst.Join(entry.Base()), force); err != nil {
				return err
			}
		}
		return nil
	}

	if dst.Exist() && !force {
		slog.Info("model artifact already present; keeping existing copy", slog.String("path", dst.String()))
		return nil
	}
	if err := dst.Parent().MkdirAll(); err != nil {
		return err
	}
	if dst.Exist() {
		if err := dst.RemoveAll(); err != nil {
			return err
		}
	}
	return src.CopyTo(dst)
}

// localizeInstalledRelease converts an extracted release into a ready-to-run app on this
// machine:
//   - it produces the standard .cache/app-compose.yaml from the relocatable frozen compose
//     by resolving the app path and custom-models dir for this destination (the ${HOST_IP}
//     placeholder is left for the run command to fill, as for any app);
//   - it stamps the release metadata (release number) into app.yaml so the normal run
//     command launches it as a release (no recompilation / re-provisioning).
//
// The manifest file (arduino-app-release.yaml) extracted next to app.yaml is preserved.
func localizeInstalledRelease(ctx context.Context, cfg config.Configuration, bricksIndex *bricksindex.BricksIndex, modelsIndex *modelsindex.ModelsIndex, plat platform.Platform, manifest *ReleaseManifest, extractedAppDir *paths.Path, finalAppPath *paths.Path) error {
	cacheDir := extractedAppDir.Join(".cache")
	frozen := cacheDir.Join("release-compose.yaml")
	if frozen.NotExist() {
		return fmt.Errorf("%w: release is missing its compose file (%s)", ErrBadRequest, frozen.Base())
	}

	content, err := frozen.ReadFile()
	if err != nil {
		return fmt.Errorf("reading frozen compose: %w", err)
	}
	localized := string(content)
	// Resolve the stable, host-specific paths now.
	localized = strings.ReplaceAll(localized, appHomePlaceholder, finalAppPath.String())
	localized = strings.ReplaceAll(localized, customModelDirPlaceholder, cfg.CustomModelsDir().String())

	// Resolve the remaining host-specific env placeholders (HOST_IP, MODELS_PATH,
	// CUSTOM_MODEL_PATH) to this machine's values, so the installed compose looks like a
	// normally provisioned one and read-only commands (`app logs`, `app stop`) don't warn
	// about unset variables. HOST_IP keeps a ${HOST_IP:-<ip>} form so each `app start` still
	// re-resolves the current address while offline commands fall back to a concrete value.
	localized = localizeHostSpecificEnv(ctx, localized, cfg, bricksIndex, modelsIndex, plat, extractedAppDir)

	if err := cacheDir.Join("app-compose.yaml").WriteFile([]byte(localized)); err != nil {
		return fmt.Errorf("writing app compose: %w", err)
	}
	_ = frozen.Remove() // superseded by app-compose.yaml

	// Stamp the release metadata into app.yaml.
	descriptorPath := extractedAppDir.Join("app.yaml")
	descriptor, err := app.ParseDescriptorFile(descriptorPath)
	if err != nil {
		return fmt.Errorf("reading app.yaml: %w", err)
	}
	release := ""
	if manifest != nil {
		release = manifest.Release
	}
	descriptor.FrozenRelease = &app.FrozenReleaseInfo{Number: release}
	out, err := yaml.Marshal(descriptor)
	if err != nil {
		return fmt.Errorf("marshaling app.yaml: %w", err)
	}
	if err := descriptorPath.WriteFile(out); err != nil {
		return fmt.Errorf("writing app.yaml: %w", err)
	}
	return nil
}

// localizeHostSpecificEnv replaces the ${HOST_IP}/${MODELS_PATH}/${CUSTOM_MODEL_PATH}
// placeholders that tokenizeHostSpecificEnv left in the frozen compose with this machine's
// concrete values, computed the same way a normal `app start` would (getAppEnvironmentVariables).
// The stable ones (paths) become literals; HOST_IP becomes a ${HOST_IP:-<ip>} default so it
// still refreshes at each start but never renders empty for offline commands. If the app or
// its environment cannot be resolved, the placeholders are left as-is (they are still filled
// at run time) rather than failing the install.
func localizeHostSpecificEnv(ctx context.Context, compose string, cfg config.Configuration, bricksIndex *bricksindex.BricksIndex, modelsIndex *modelsindex.ModelsIndex, plat platform.Platform, extractedAppDir *paths.Path) string {
	installedApp, err := app.Load(extractedAppDir)
	if err != nil {
		slog.Warn("could not load app to localize release compose env; leaving placeholders", slog.String("error", err.Error()))
		return compose
	}
	envs := getAppEnvironmentVariables(ctx, installedApp, bricksIndex, modelsIndex, plat, cfg)
	return applyHostEnvDefaults(compose, envs)
}

// applyHostEnvDefaults rewrites each ${KEY} host-specific placeholder to its concrete value
// from envs. Path values become literals (they are stable on this machine); HOST_IP becomes a
// ${HOST_IP:-<ip>} default so `app start` still refreshes it from the run environment while
// offline commands resolve it to a concrete address instead of a blank string. Keys missing
// from envs are left as bare placeholders (still filled at run time).
func applyHostEnvDefaults(compose string, envs helpers.EnvVars) string {
	for _, key := range hostSpecificEnvKeys {
		v := envs[key]
		if v == "" {
			continue
		}
		replacement := v
		if key == "HOST_IP" {
			replacement = "${" + key + ":-" + v + "}"
		}
		compose = strings.ReplaceAll(compose, "${"+key+"}", replacement)
	}
	return compose
}

// renderSecretsTemplate produces the content of a data/secrets.env template listing the
// secrets the user must provide, so they know what to fill in.
func renderSecretsTemplate(required []ReleaseSecret) []byte {
	var b strings.Builder
	b.WriteString("# Secrets required by this Arduino App.\n")
	b.WriteString("# Fill in the values below, then start the app with `arduino-app-cli app start`.\n")
	b.WriteString("# These values are NOT included in the release/export and stay on this machine.\n\n")
	for _, s := range required {
		if s.Brick != "" {
			b.WriteString(fmt.Sprintf("# required by brick %s\n", s.Brick))
		}
		b.WriteString(s.Name + "=\n")
	}
	return []byte(b.String())
}

// writeSecretsTemplate creates a data/secrets.env template listing the secrets the release
// requires, so the user knows what to fill in. It never overwrites an existing file.
func writeSecretsTemplate(manifest *ReleaseManifest, extractedAppDir *paths.Path) error {
	if manifest == nil || len(manifest.RequiredSecrets) == 0 {
		return nil
	}
	dest := extractedAppDir.Join("data", "secrets.env")
	if dest.Exist() {
		return nil
	}
	if err := dest.Parent().MkdirAll(); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}
	if err := dest.WriteFile(renderSecretsTemplate(manifest.RequiredSecrets)); err != nil {
		return fmt.Errorf("writing secrets template: %w", err)
	}
	return nil
}

// secretPlaceholderRE matches an env value that is exactly a ${NAME} reference, i.e. a
// secret that was scrubbed at build time and must be supplied on this machine.
var secretPlaceholderRE = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*\}$`)

// declaredSecretNames returns the authoritative set of secret variable names for an
// installed app: those declared in its release manifest (a frozen release) and/or listed in
// its data/secrets.env template (a scrubbed export). Driving secret resolution from this
// declared set — rather than from any ${NAME}-shaped env value — avoids misclassifying a
// legitimate non-secret ${...} value as a required secret.
func declaredSecretNames(arduinoApp app.ArduinoApp) []string {
	seen := make(map[string]struct{})
	var names []string
	add := func(n string) {
		if n == "" {
			return
		}
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			names = append(names, n)
		}
	}

	if mp := arduinoApp.ReleaseManifestPath(); mp != nil && mp.Exist() {
		if data, err := mp.ReadFile(); err == nil {
			var m ReleaseManifest
			if err := yaml.Unmarshal(data, &m); err == nil {
				for _, s := range m.RequiredSecrets {
					add(s.Name)
				}
			} else {
				slog.Warn("failed to parse release manifest for secret names", slog.String("path", mp.String()), slog.Any("error", err))
			}
		}
	}

	if sf := arduinoApp.SecretsEnvFilePath(); sf != nil && sf.Exist() {
		if parsed, err := parseEnvFile(sf.String()); err == nil {
			for k := range parsed {
				add(k)
			}
		}
	}

	return names
}

// applyReleaseSecrets resolves the scrubbed secrets of a released app from its
// data/secrets.env file and injects the values into envs (so `docker compose up` can
// interpolate the ${NAME} placeholders in the frozen compose). The set of required secrets
// is taken from the app's release manifest / secrets.env template (see declaredSecretNames);
// only declared secrets that are actually referenced by this app's env (still a ${NAME}
// placeholder) are resolved. It returns the names of secrets that are still missing/empty so
// the caller can fail with a clear message. It never downloads anything.
func applyReleaseSecrets(arduinoApp app.ArduinoApp, envs helpers.EnvVars) ([]string, error) {
	declared := declaredSecretNames(arduinoApp)
	// Fallback: infer required secrets from env values still shaped like ${NAME}. This is only
	// safe for release/exported apps (whose secrets were deliberately scrubbed to placeholders);
	// applying it to a normal app would misclassify a legitimate compose passthrough such as
	// `TZ: ${TZ}` as a missing required secret and refuse to start. Restrict it to frozen
	// releases, which is the only case where declaredSecretNames can come up empty yet the
	// compose legitimately still contains ${NAME} secret placeholders (e.g. an unreadable
	// manifest).
	if len(declared) == 0 && arduinoApp.IsFrozenRelease() {
		for name, value := range envs {
			if secretPlaceholderRE.MatchString(value) {
				declared = append(declared, name)
			}
		}
	}
	if len(declared) == 0 {
		return nil, nil
	}

	values := map[string]string{}
	secretsFile := arduinoApp.SecretsEnvFilePath()
	if secretsFile.Exist() {
		parsed, err := parseEnvFile(secretsFile.String())
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", secretsFile, err)
		}
		values = parsed
	}

	var missing []string
	for _, name := range declared {
		// Only act on declared secrets this app actually references and hasn't resolved yet.
		cur, inEnv := envs[name]
		if !inEnv || !secretPlaceholderRE.MatchString(cur) {
			continue
		}
		if v, ok := values[name]; ok && v != "" {
			envs[name] = v
		} else {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing, nil
}

// parseEnvFile reads a minimal KEY=VALUE env file (blank lines and #-comments ignored).
// Surrounding single/double quotes around the value are stripped.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path) // nolint:gosec
	if err != nil {
		return nil, err
	}
	defer f.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if key != "" {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// readReleaseMetadata scans the archive for its root prefix and parses the release
// manifest. It validates the archive really is an app release.
func readReleaseMetadata(tarPath *paths.Path) (string, *ReleaseManifest, error) {
	f, err := os.Open(tarPath.String())
	if err != nil {
		return "", nil, fmt.Errorf("unable to open release: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", nil, fmt.Errorf("not a valid .tar.gz release: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var manifest *ReleaseManifest
	rootPrefix := ""
	foundManifest := false
	// The real manifest lives at the archive root ("<root>/arduino-app-release.yaml" or the
	// bare root). Pick the shallowest match so a decoy manifest planted deeper in the tree
	// (e.g. under models/) cannot hijack the root prefix, regardless of entry order.
	bestDepth := int(^uint(0) >> 1)

	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", nil, fmt.Errorf("reading release: %w", err)
		}
		name := filepath.ToSlash(filepath.Clean(header.Name))
		if filepath.Base(name) != ReleaseManifestFileName {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(name))
		depth := 0
		if dir != "." {
			depth = strings.Count(dir, "/") + 1
		}
		if depth >= bestDepth {
			continue // a shallower (or equal, already-taken) manifest wins
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxManifestSize))
		if err != nil {
			return "", nil, fmt.Errorf("reading manifest: %w", err)
		}
		var m ReleaseManifest
		if err := yaml.Unmarshal(data, &m); err != nil {
			return "", nil, fmt.Errorf("invalid manifest: %w", err)
		}
		manifest = &m
		if dir != "." {
			rootPrefix = dir
		} else {
			rootPrefix = ""
		}
		bestDepth = depth
		foundManifest = true
	}

	if !foundManifest {
		return "", nil, fmt.Errorf("%s not found in archive: not an Arduino App Release", ReleaseManifestFileName)
	}
	return rootPrefix, manifest, nil
}

// archiveRelativeName cleans a raw tar entry name and, if rootPrefixSlash is set, strips
// it, reporting ok=false for entries outside rootPrefix (to be skipped by the caller).
func archiveRelativeName(rawName, rootPrefixSlash string) (name string, ok bool) {
	name = filepath.ToSlash(filepath.Clean(rawName))
	if rootPrefixSlash != "" && rootPrefixSlash != "." {
		if name != rootPrefixSlash && !strings.HasPrefix(name, rootPrefixSlash+"/") {
			return "", false
		}
		name = strings.TrimPrefix(name, rootPrefixSlash)
		name = strings.TrimPrefix(name, "/")
	}
	if name == "" || name == "." {
		return "", false
	}
	return name, true
}

// collectArchiveNames does a first pass over the archive to gather every entry's
// (rootPrefix-stripped) name, used to check whether an absolute symlink target would be
// treated as a directory by another entry later in the same archive.
func collectArchiveNames(tarPath *paths.Path, rootPrefixSlash string) ([]string, error) {
	f, err := os.Open(tarPath.String())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	var names []string
	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading release entry: %w", err)
		}
		if name, ok := archiveRelativeName(header.Name, rootPrefixSlash); ok {
			names = append(names, name)
		}
	}
	return names, nil
}

// nestedUnder reports whether some other entry in names is nested under name, i.e. name
// would be traversed as a directory component to reach it.
func nestedUnder(names []string, name string) bool {
	prefix := name + "/"
	for _, n := range names {
		if n != name && strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

// realParentWithinDest verifies that target's parent directory, after resolving any symlinks
// already present on disk, is still inside dest. The lexical destClean-prefix check on target
// only inspects the literal path; this catches writes that would traverse a symlinked path
// component (e.g. a relative symlink pointing at an earlier absolute symlink) to escape dest.
// The parent directory must already exist when this is called.
func realParentWithinDest(target, destClean string) error {
	realParent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return fmt.Errorf("resolving parent of release entry %s: %w", target, err)
	}
	if !strings.HasPrefix(realParent+string(os.PathSeparator), destClean) {
		return fmt.Errorf("illegal release entry (escapes destination via a symlinked path): %s", target)
	}
	return nil
}

// extractTarGz extracts the archive into dest, stripping rootPrefix, with protection
// against path traversal and oversized files. Symlinks are preserved (needed by the venv).
func extractTarGz(tarPath *paths.Path, dest *paths.Path, rootPrefix string) error {
	rootPrefixSlash := filepath.ToSlash(rootPrefix)

	// First pass: gather all entry names so absolute symlinks can be checked against being
	// used as a directory by a later entry (which would let it write outside dest).
	allNames, err := collectArchiveNames(tarPath, rootPrefixSlash)
	if err != nil {
		return err
	}

	f, err := os.Open(tarPath.String())
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	destClean := filepath.Clean(dest.String()) + string(os.PathSeparator)

	entryCount := 0
	var totalWritten int64

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading release entry: %w", err)
		}

		entryCount++
		if entryCount > maxReleaseEntries {
			return fmt.Errorf("release has too many entries (limit %d)", maxReleaseEntries)
		}

		name, ok := archiveRelativeName(header.Name, rootPrefixSlash)
		if !ok {
			continue
		}

		target := filepath.Join(dest.String(), filepath.FromSlash(name))
		if !strings.HasPrefix(target, destClean) {
			return fmt.Errorf("illegal file path in release: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent directory: %w", err)
			}
			if err := realParentWithinDest(target, destClean); err != nil {
				return err
			}
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory %s: %w", target, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent directory: %w", err)
			}
			if err := realParentWithinDest(target, destClean); err != nil {
				return err
			}
			// Absolute targets are only safe when nothing else in the archive is nested
			// under this entry's path: otherwise a later entry could be written through
			// it to an arbitrary host location. The Python venv legitimately needs
			// absolute symlinks (bin/python -> the interpreter baked into the base
			// image), so a bare absolute leaf symlink is allowed; relative targets must
			// still resolve inside dest.
			if filepath.IsAbs(header.Linkname) {
				if nestedUnder(allNames, name) {
					return fmt.Errorf("illegal absolute symlink target in release: %s -> %s", header.Name, header.Linkname)
				}
			} else {
				resolved := filepath.Join(filepath.Dir(target), filepath.FromSlash(header.Linkname))
				if !strings.HasPrefix(resolved, destClean) {
					return fmt.Errorf("illegal symlink target in release: %s -> %s", header.Name, header.Linkname)
				}
			}
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("create symlink %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent directory: %w", err)
			}
			if err := realParentWithinDest(target, destClean); err != nil {
				return err
			}
			// Remove any pre-existing entry (e.g. a symlink planted by an earlier entry with
			// the same name) so we never truncate/write *through* it to an arbitrary path.
			// O_EXCL then guarantees a fresh, non-followed regular file at this path.
			_ = os.Remove(target)
			outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			written, err := io.Copy(outFile, io.LimitReader(tr, maxReleaseFileSize+1))
			outFile.Close()
			if err != nil {
				return fmt.Errorf("write file %s: %w", target, err)
			}
			if written > maxReleaseFileSize {
				return fmt.Errorf("file %s too large", header.Name)
			}
			totalWritten += written
			if totalWritten > maxReleaseTotalSize {
				return fmt.Errorf("release too large when uncompressed (limit %d bytes)", maxReleaseTotalSize)
			}
		default:
			slog.Debug("skipping unsupported tar entry", slog.String("name", header.Name), slog.Int("type", int(header.Typeflag)))
		}
	}
	return nil
}

// uploadPrebuiltSketch flashes the already-compiled sketch binary found in the app's
// build cache to the microcontroller. It mirrors the upload tail of compileUploadSketch
// but never invokes the compiler.
func uploadPrebuiltSketch(
	ctx context.Context,
	plat platform.Platform,
	arduinoApp app.ArduinoApp,
	w io.Writer,
) error {
	sketchPath, ok := arduinoApp.GetSketchPath()
	if !ok {
		return fmt.Errorf("no sketch path found in the Arduino app")
	}
	buildPath := arduinoApp.SketchBuildPath()
	if buildPath.NotExist() {
		return fmt.Errorf("%w: prebuilt sketch not found at %s", ErrNotFrozenRelease, buildPath.String())
	}

	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return err
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	sketchResp, err := srv.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath.String()})
	if err != nil {
		return err
	}
	profile := sketchResp.GetSketch().GetDefaultProfile().GetName()
	if profile == "" {
		return fmt.Errorf("sketch %q has no default profile", sketchPath)
	}

	if err := srv.Init(
		&rpc.InitRequest{Instance: inst, SketchPath: sketchPath.String(), Profile: profile},
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			slog.Debug("Arduino init instance", slog.String("instance", r.String()))
			return nil
		}),
	); err != nil {
		return err
	}

	menuOptions, err := GetPlatformMenuOptions(ctx, plat)
	if err != nil {
		slog.Warn("failed to get platform menu options", slog.String("error", err.Error()))
	}

	// Support the legacy ram upload option if there isn't the new wait_linux_boot option.
	if !menuOptions.Has(WaitForApp) && plat.SupportFlashToRam() {
		if err := legacyUploadSketchInRam(ctx, w, srv, inst, plat, sketchPath.String(), buildPath.String()); err != nil {
			slog.Warn("failed to upload in ram mode, retrying after configuring ram mode", slog.String("error", err.Error()))
			if err := configureMicroInRamMode(ctx, w, srv, inst, plat); err != nil {
				return err
			}
			return legacyUploadSketchInRam(ctx, w, srv, inst, plat, sketchPath.String(), buildPath.String())
		}
		return nil
	}

	stream, _ := commands.UploadToServerStreams(ctx, w, w)
	return srv.Upload(&rpc.UploadRequest{
		Instance:   inst,
		Fqbn:       plat.FQBN,
		SketchPath: sketchPath.String(),
		ImportDir:  buildPath.String(),
	}, stream)
}
