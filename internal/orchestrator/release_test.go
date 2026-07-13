// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestReleaseRootFolderName(t *testing.T) {
	require.Equal(t, "my-app", releaseRootFolderName("My App"))
	require.Equal(t, "smart-garden", releaseRootFolderName("Smart Garden"))
	require.Equal(t, "app-release", releaseRootFolderName(""))
}

func TestExtractComposeImages(t *testing.T) {
	compose := []byte(`
name: demo
services:
  main:
    image: ghcr.io/arduino/app-bricks/python-apps-base:0.11.0rc6
  db:
    image: postgres:16
  db2:
    image: postgres:16
`)
	images := extractComposeImages(compose)
	require.ElementsMatch(t, []string{
		"ghcr.io/arduino/app-bricks/python-apps-base:0.11.0rc6",
		"postgres:16",
	}, images)
}

// writeFile is a small helper to create a file with content, making parents as needed.
func writeFile(t *testing.T, p *paths.Path, content string) {
	t.Helper()
	require.NoError(t, p.Parent().MkdirAll())
	require.NoError(t, p.WriteFile([]byte(content)))
}

func TestWriteAndExtractTarGzRoundTrip(t *testing.T) {
	src := paths.New(t.TempDir())

	// A representative app layout.
	writeFile(t, src.Join("app.yaml"), "name: Demo App\n")
	writeFile(t, src.Join("python", "main.py"), "print('hi')\n")
	writeFile(t, src.Join("sketch", "sketch.ino"), "void setup(){}\n")
	writeFile(t, src.Join(".cache", "sketch", "sketch.ino.bin"), "BINARY")
	writeFile(t, src.Join(".cache", "venv", "pyvenv.cfg"), "home = /usr\n")
	// These must be excluded by the packer:
	writeFile(t, src.Join("data", "secret.db"), "PRIVATE")
	writeFile(t, src.Join(".cache", "app-compose.yaml"), "name: host-baked\n")
	writeFile(t, src.Join(".cache", "app-compose-overrides.yaml"), "services: {}\n")

	out := paths.New(t.TempDir()).Join("demo.tar.gz")

	manifest := []byte("format_version: \"1.0\"\napp_name: Demo App\nboard_name: unoq\n")
	frozen := []byte("name: demo\nservices:\n  main:\n    image: x:1\n")
	// app.yaml is provided via extraFiles (scrubbed), not from the walk.
	extra := map[string][]byte{
		"app.yaml":                    []byte("name: Demo App\n"),
		ReleaseManifestFileName:       manifest,
		".cache/release-compose.yaml": frozen,
	}

	require.NoError(t, writeAppTarGz(src, "demo", out, extra, nil))
	require.True(t, out.Exist())

	// Metadata reads back the manifest and detects the root prefix.
	rootPrefix, m, err := readReleaseMetadata(out)
	require.NoError(t, err)
	require.Equal(t, "demo", rootPrefix)
	require.Equal(t, "Demo App", m.AppName)
	require.Equal(t, "unoq", m.BoardName)

	// Extract and verify contents.
	dest := paths.New(t.TempDir()).Join("extracted")
	require.NoError(t, dest.MkdirAll())
	require.NoError(t, extractTarGz(out, dest, rootPrefix))

	requireFileContent(t, dest.Join("app.yaml"), "name: Demo App\n")
	requireFileContent(t, dest.Join("python", "main.py"), "print('hi')\n")
	requireFileContent(t, dest.Join(".cache", "sketch", "sketch.ino.bin"), "BINARY")
	requireFileContent(t, dest.Join(".cache", "venv", "pyvenv.cfg"), "home = /usr\n")
	requireFileContent(t, dest.Join(ReleaseManifestFileName), string(manifest))
	requireFileContent(t, dest.Join(".cache", "release-compose.yaml"), string(frozen))

	// Excluded entries must not be present.
	require.False(t, dest.Join("data", "secret.db").Exist(), "data/ must be excluded")
	require.False(t, dest.Join(".cache", "app-compose.yaml").Exist(), "host-baked compose must be excluded")
	require.False(t, dest.Join(".cache", "app-compose-overrides.yaml").Exist(), "host-baked override must be excluded")
}

// rawTarEntry describes a single tar entry built directly (bypassing the filesystem walk)
// so tests can craft archives an attacker could produce, e.g. a symlink nested by another
// entry, without ever touching the real filesystem outside the test's temp dirs.
type rawTarEntry struct {
	name     string
	linkname string
	typeflag byte
	content  string
}

func writeRawTarGz(t *testing.T, out *paths.Path, entries []rawTarEntry) {
	t.Helper()
	require.NoError(t, out.Parent().MkdirAll())
	f, err := os.Create(out.String())
	require.NoError(t, err)
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		mode := int64(0o644)
		if e.typeflag == tar.TypeSymlink {
			mode = 0o777
		}
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     e.name,
			Linkname: e.linkname,
			Typeflag: e.typeflag,
			Mode:     mode,
			Size:     int64(len(e.content)),
		}))
		if e.content != "" {
			_, err := tw.Write([]byte(e.content))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
}

const testManifest = "format_version: \"1.0\"\napp_name: Demo App\nboard_name: unoq\n"

func TestExtractTarGzAllowsLeafAbsoluteSymlink(t *testing.T) {
	// A Python venv's bin/python is legitimately an absolute symlink to the interpreter
	// baked into the base image; install must not reject it.
	out := paths.New(t.TempDir()).Join("demo.tar.gz")
	writeRawTarGz(t, out, []rawTarEntry{
		{name: "demo/" + ReleaseManifestFileName, typeflag: tar.TypeReg, content: testManifest},
		{name: "demo/.cache/venv/bin/python", typeflag: tar.TypeSymlink, linkname: "/usr/local/bin/python"},
	})

	rootPrefix, _, err := readReleaseMetadata(out)
	require.NoError(t, err)

	dest := paths.New(t.TempDir()).Join("extracted")
	require.NoError(t, dest.MkdirAll())
	require.NoError(t, extractTarGz(out, dest, rootPrefix))

	target, err := os.Readlink(dest.Join(".cache", "venv", "bin", "python").String())
	require.NoError(t, err)
	require.Equal(t, "/usr/local/bin/python", target)
}

func TestExtractTarGzRejectsAbsoluteSymlinkUsedAsDirectory(t *testing.T) {
	// An absolute symlink that a later entry nests under would let install write outside
	// dest (e.g. onto /usr/local/bin on the host) once the OS resolves the path.
	out := paths.New(t.TempDir()).Join("demo.tar.gz")
	writeRawTarGz(t, out, []rawTarEntry{
		{name: "demo/" + ReleaseManifestFileName, typeflag: tar.TypeReg, content: testManifest},
		{name: "demo/.cache/escape", typeflag: tar.TypeSymlink, linkname: "/usr/local/bin"},
		{name: "demo/.cache/escape/evil", typeflag: tar.TypeReg, content: "PAYLOAD"},
	})

	rootPrefix, _, err := readReleaseMetadata(out)
	require.NoError(t, err)

	dest := paths.New(t.TempDir()).Join("extracted")
	require.NoError(t, dest.MkdirAll())
	err = extractTarGz(out, dest, rootPrefix)
	require.Error(t, err)
	require.Contains(t, err.Error(), "illegal absolute symlink target")
}

func TestExtractTarGzRejectsEscapingRelativeSymlink(t *testing.T) {
	out := paths.New(t.TempDir()).Join("demo.tar.gz")
	writeRawTarGz(t, out, []rawTarEntry{
		{name: "demo/" + ReleaseManifestFileName, typeflag: tar.TypeReg, content: testManifest},
		{name: "demo/escape", typeflag: tar.TypeSymlink, linkname: "../../../etc/passwd"},
	})

	rootPrefix, _, err := readReleaseMetadata(out)
	require.NoError(t, err)

	dest := paths.New(t.TempDir()).Join("extracted")
	require.NoError(t, dest.MkdirAll())
	err = extractTarGz(out, dest, rootPrefix)
	require.Error(t, err)
	require.Contains(t, err.Error(), "illegal symlink target")
}

func TestExtractTarGzRejectsRegularFileThroughPlantedSymlink(t *testing.T) {
	// A leaf absolute symlink is allowed, but a later regular-file entry with the SAME name
	// must not be written *through* it (which would overwrite an arbitrary host file).
	victim := paths.New(t.TempDir()).Join("victim.txt")
	require.NoError(t, victim.WriteFile([]byte("ORIGINAL")))

	out := paths.New(t.TempDir()).Join("demo.tar.gz")
	writeRawTarGz(t, out, []rawTarEntry{
		{name: "demo/" + ReleaseManifestFileName, typeflag: tar.TypeReg, content: testManifest},
		{name: "demo/pwn", typeflag: tar.TypeSymlink, linkname: victim.String()},
		{name: "demo/pwn", typeflag: tar.TypeReg, content: "PWNED"},
	})

	rootPrefix, _, err := readReleaseMetadata(out)
	require.NoError(t, err)

	dest := paths.New(t.TempDir()).Join("extracted")
	require.NoError(t, dest.MkdirAll())
	require.NoError(t, extractTarGz(out, dest, rootPrefix)) // remove-before-create makes it a no-op write

	got, err := os.ReadFile(victim.String())
	require.NoError(t, err)
	require.Equal(t, "ORIGINAL", string(got), "host file must not be overwritten through the symlink")
}

func TestExtractTarGzRejectsRelativeToAbsoluteSymlinkChain(t *testing.T) {
	// escape -> /abs (absolute, allowed), link -> escape (relative, lexically inside dest),
	// then link/f: writing it must NOT traverse the chain to /abs.
	outside := paths.New(t.TempDir()) // an existing absolute dir outside dest
	out := paths.New(t.TempDir()).Join("demo.tar.gz")
	writeRawTarGz(t, out, []rawTarEntry{
		{name: "demo/" + ReleaseManifestFileName, typeflag: tar.TypeReg, content: testManifest},
		{name: "demo/escape", typeflag: tar.TypeSymlink, linkname: outside.String()},
		{name: "demo/link", typeflag: tar.TypeSymlink, linkname: "escape"},
		{name: "demo/link/f", typeflag: tar.TypeReg, content: "PWNED"},
	})

	rootPrefix, _, err := readReleaseMetadata(out)
	require.NoError(t, err)

	dest := paths.New(t.TempDir()).Join("extracted")
	require.NoError(t, dest.MkdirAll())
	err = extractTarGz(out, dest, rootPrefix)
	require.Error(t, err)
	require.Contains(t, err.Error(), "escapes destination")
	require.False(t, outside.Join("f").Exist(), "file must not be written outside dest")
}

func TestReadReleaseMetadataRejectsNonRelease(t *testing.T) {
	// A tar.gz without a manifest is not a release.
	src := paths.New(t.TempDir())
	writeFile(t, src.Join("app.yaml"), "name: NoManifest\n")
	out := paths.New(t.TempDir()).Join("plain.tar.gz")
	require.NoError(t, writeAppTarGz(src, "plain", out, nil, nil))

	_, _, err := readReleaseMetadata(out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not an Arduino App Release")
}

func TestReadReleaseMetadataPrefersShallowestManifest(t *testing.T) {
	// A decoy manifest planted deeper in the tree (and appearing first) must not hijack the
	// root prefix; the shallowest manifest wins regardless of order.
	out := paths.New(t.TempDir()).Join("demo.tar.gz")
	writeRawTarGz(t, out, []rawTarEntry{
		{name: "app/models/decoy/" + ReleaseManifestFileName, typeflag: tar.TypeReg, content: "app_name: DECOY\n"},
		{name: "app/" + ReleaseManifestFileName, typeflag: tar.TypeReg, content: "app_name: REAL\n"},
	})
	rootPrefix, m, err := readReleaseMetadata(out)
	require.NoError(t, err)
	require.Equal(t, "app", rootPrefix)
	require.Equal(t, "REAL", m.AppName)
}

func TestModelBundlingAndRestore(t *testing.T) {
	// Source config: a custom-models dir holding a fake Edge-Impulse model.
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", t.TempDir())
	srcModelsDir := t.TempDir()
	t.Setenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR", srcModelsDir)
	srcCfg, err := config.NewFromEnv()
	require.NoError(t, err)

	const modelRel = "custom-ei/ei-model-1-2"
	modelFolder := srcCfg.CustomModelsDir().Join("custom-ei", "ei-model-1-2")
	writeFile(t, modelFolder.Join("model.eim"), "MODELBLOB")
	writeFile(t, modelFolder.Join("model.yaml"), "name: EI\n")

	// Build a release that bundles that model folder.
	appSrc := paths.New(t.TempDir())
	writeFile(t, appSrc.Join("app.yaml"), "name: Model App\n")
	writeFile(t, appSrc.Join("python", "main.py"), "x=1\n")

	out := paths.New(t.TempDir()).Join("modelapp.tar.gz")
	extra := map[string][]byte{ReleaseManifestFileName: []byte("app_name: Model App\n")}
	modelArtifacts := []tarPath{{archiveRel: releaseModelsDirPrefix + "/" + modelRel, source: modelFolder}}
	require.NoError(t, writeAppTarGz(appSrc, "modelapp", out, extra, modelArtifacts))

	// Extract and confirm the model rides along under models/<rel>.
	extracted := paths.New(t.TempDir()).Join("ex")
	require.NoError(t, extracted.MkdirAll())
	require.NoError(t, extractTarGz(out, extracted, "modelapp"))
	requireFileContent(t, extracted.Join("models", "custom-ei", "ei-model-1-2", "model.eim"), "MODELBLOB")

	// Restore into a fresh destination custom-models dir.
	t.Setenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR", t.TempDir())
	destCfg, err := config.NewFromEnv()
	require.NoError(t, err)
	manifest := &ReleaseManifest{Models: []ReleaseModel{{ID: "ei-model-1-2", Bundled: true, Paths: []string{modelRel}}}}
	require.NoError(t, restoreBundledModels(destCfg, manifest, extracted, false))

	requireFileContent(t, destCfg.CustomModelsDir().Join("custom-ei", "ei-model-1-2", "model.eim"), "MODELBLOB")
	requireFileContent(t, destCfg.CustomModelsDir().Join("custom-ei", "ei-model-1-2", "model.yaml"), "name: EI\n")

	// Restoring again must not fail and must keep the existing copy (no force).
	require.NoError(t, restoreBundledModels(destCfg, manifest, extracted, false))
}

func TestRestoreBundledModelsRejectsTraversalPath(t *testing.T) {
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", t.TempDir())
	t.Setenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR", t.TempDir())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)

	extracted := paths.New(t.TempDir())
	// Plant a file the traversal would point at, to prove it is NOT copied out.
	writeFile(t, extracted.Join("secret.txt"), "SENSITIVE")

	manifest := &ReleaseManifest{Models: []ReleaseModel{{ID: "evil", Bundled: true, Paths: []string{"../secret.txt"}}}}
	err = restoreBundledModels(cfg, manifest, extracted, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "illegal artifact path")
	require.False(t, cfg.CustomModelsDir().Parent().Join("secret.txt").Exist())
}

func TestRestoreBundledModelsFailsOnMissingArtifact(t *testing.T) {
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", t.TempDir())
	t.Setenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR", t.TempDir())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)

	extracted := paths.New(t.TempDir()) // no models/ payload
	manifest := &ReleaseManifest{Models: []ReleaseModel{{ID: "m", Bundled: true, Paths: []string{"custom-ei/m"}}}}
	err = restoreBundledModels(cfg, manifest, extracted, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing from the release archive")
}

func TestLocalizeInstalledRelease(t *testing.T) {
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", t.TempDir())
	t.Setenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR", t.TempDir())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)

	extracted := paths.New(t.TempDir())
	writeFile(t, extracted.Join("app.yaml"), "name: Demo\ndescription: d\n")
	writeFile(t, extracted.Join("python", "main.py"), "x=1\n")
	frozen := "name: demo\n" +
		"services:\n" +
		"  main:\n" +
		"    environment:\n" +
		"      HOST_IP: ${HOST_IP}\n" +
		"    volumes:\n" +
		"      - ${APP_HOME}:/app\n" +
		"      - ${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}/edge-impulse:/models\n"
	writeFile(t, extracted.Join(".cache", "release-compose.yaml"), frozen)

	finalPath := paths.New(t.TempDir()).Join("my-app")
	manifest := &ReleaseManifest{Release: "20260101120000"}
	require.NoError(t, localizeInstalledRelease(context.Background(), cfg, &bricksindex.BricksIndex{}, &modelsindex.ModelsIndex{}, platform.Platform{}, manifest, extracted, finalPath))

	// The staging compose is gone; the standard one exists with resolved paths.
	require.False(t, extracted.Join(".cache", "release-compose.yaml").Exist())
	composeBytes, err := os.ReadFile(extracted.Join(".cache", "app-compose.yaml").String())
	require.NoError(t, err)
	compose := string(composeBytes)
	require.Contains(t, compose, finalPath.String()+":/app")
	require.Contains(t, compose, cfg.CustomModelsDir().String()+"/edge-impulse:/models")
	require.Contains(t, compose, "${HOST_IP")     // kept dynamic (bare or with a :-default) for the run command
	require.NotContains(t, compose, "${APP_HOME}") // resolved at install time

	// app.yaml is stamped as a frozen release with the release number.
	desc, err := app.ParseDescriptorFile(extracted.Join("app.yaml"))
	require.NoError(t, err)
	require.NotNil(t, desc.FrozenRelease)
	require.Equal(t, "20260101120000", desc.FrozenRelease.Number)
}

func TestScrubAppSecrets(t *testing.T) {
	desc := app.AppDescriptor{
		Bricks: []app.Brick{
			{ID: "arduino:db", Variables: map[string]string{"DB_PASSWORD": "s3cr3t", "DB_HOST": "localhost"}},
			{ID: "arduino:weather", Variables: map[string]string{"API_KEY": "abc123"}},
			{ID: "arduino:empty", Variables: map[string]string{"OPTIONAL_SECRET": ""}},
		},
	}
	idx := &bricksindex.BricksIndex{BuiltInBricks: []bricksindex.Brick{
		{ID: "arduino:db", Variables: []bricksindex.BrickVariable{
			{Name: "DB_PASSWORD", Secret: true},
			{Name: "DB_HOST"}, // not secret
		}},
		{ID: "arduino:weather", Variables: []bricksindex.BrickVariable{{Name: "API_KEY", Secret: true}}},
		{ID: "arduino:empty", Variables: []bricksindex.BrickVariable{{Name: "OPTIONAL_SECRET", Secret: true}}},
	}}

	required, values := scrubAppSecrets(&desc, idx)

	// Non-empty secret values are replaced with ${NAME} placeholders.
	require.Equal(t, "${DB_PASSWORD}", desc.Bricks[0].Variables["DB_PASSWORD"])
	require.Equal(t, "${API_KEY}", desc.Bricks[1].Variables["API_KEY"])
	// Non-secret and empty-secret variables are left untouched.
	require.Equal(t, "localhost", desc.Bricks[0].Variables["DB_HOST"])
	require.Equal(t, "", desc.Bricks[2].Variables["OPTIONAL_SECRET"])

	names := make([]string, 0, len(required))
	for _, s := range required {
		names = append(names, s.Name)
	}
	require.ElementsMatch(t, []string{"DB_PASSWORD", "API_KEY"}, names)

	// The returned values map carries the scrubbed plaintext (used to also strip the
	// values from the frozen compose); non-secret and empty-secret vars are not captured.
	require.Equal(t, map[string][]string{"DB_PASSWORD": {"s3cr3t"}, "API_KEY": {"abc123"}}, values)
}

func TestScrubComposeSecretKeys(t *testing.T) {
	compose := "services:\n" +
		"  main:\n" +
		"    environment:\n" +
		"      DB_PASSWORD: s3cr3t\n" +
		"      OTHER: keepme\n" +
		"  legacy:\n" +
		"    environment:\n" +
		"      - API_KEY=abc123\n" +
		"      - KEEP=yes\n"
	out := scrubComposeSecretKeys(compose, []string{"DB_PASSWORD", "API_KEY"})

	require.Contains(t, out, "DB_PASSWORD: ${DB_PASSWORD}")
	require.NotContains(t, out, "s3cr3t")
	require.Contains(t, out, "- API_KEY=${API_KEY}")
	require.NotContains(t, out, "abc123")
	// Non-secret entries are untouched.
	require.Contains(t, out, "OTHER: keepme")
	require.Contains(t, out, "- KEEP=yes")
}

func TestParseEnvFile(t *testing.T) {
	dir := paths.New(t.TempDir())
	writeFile(t, dir.Join("secrets.env"), "# a comment\n\nDB_PASSWORD=s3cr3t\nAPI_KEY=\"quoted value\"\nEMPTY=\nBAD LINE\n")
	values, err := parseEnvFile(dir.Join("secrets.env").String())
	require.NoError(t, err)
	require.Equal(t, "s3cr3t", values["DB_PASSWORD"])
	require.Equal(t, "quoted value", values["API_KEY"])
	require.Equal(t, "", values["EMPTY"])
	require.NotContains(t, values, "BAD LINE")
}

func TestApplyReleaseSecrets(t *testing.T) {
	appDir := paths.New(t.TempDir())
	// secrets.env is the template written at install time: it lists every required secret
	// (declaring them), with the user having filled in only DB_PASSWORD.
	writeFile(t, appDir.Join("data", "secrets.env"), "DB_PASSWORD=fromfile\nAPI_KEY=\n")
	arduinoApp := app.ArduinoApp{FullPath: appDir}

	// DB_PASSWORD resolves from the file; API_KEY is declared but left empty -> missing.
	envs := helpers.EnvVars{
		"DB_PASSWORD": "${DB_PASSWORD}",
		"API_KEY":     "${API_KEY}",
		"HOST_IP":     "192.168.1.5", // real value, not a placeholder -> not a secret
	}
	missing, err := applyReleaseSecrets(arduinoApp, envs)
	require.NoError(t, err)
	require.Equal(t, "fromfile", envs["DB_PASSWORD"])
	require.Equal(t, "192.168.1.5", envs["HOST_IP"])
	require.Equal(t, []string{"API_KEY"}, missing)
	require.True(t, strings.HasPrefix(envs["API_KEY"], "${")) // left as placeholder when missing
}

func TestApplyReleaseSecretsIgnoresUndeclaredPlaceholders(t *testing.T) {
	appDir := paths.New(t.TempDir())
	// The manifest declares only DB_PASSWORD as a secret.
	writeFile(t, appDir.Join(ReleaseManifestFileName),
		"format_version: \"1.0\"\napp_name: X\nrequired_secrets:\n  - name: DB_PASSWORD\n")
	writeFile(t, appDir.Join("data", "secrets.env"), "DB_PASSWORD=fromfile\n")
	arduinoApp := app.ArduinoApp{FullPath: appDir}

	envs := helpers.EnvVars{
		"DB_PASSWORD": "${DB_PASSWORD}",
		"SOME_URL":    "${NOT_A_SECRET}", // legitimate placeholder, NOT a declared secret
	}
	missing, err := applyReleaseSecrets(arduinoApp, envs)
	require.NoError(t, err)
	// The undeclared ${NOT_A_SECRET} placeholder must not be treated as a missing secret.
	require.Empty(t, missing)
	require.Equal(t, "fromfile", envs["DB_PASSWORD"])
	require.Equal(t, "${NOT_A_SECRET}", envs["SOME_URL"]) // left untouched
}

func TestApplyReleaseSecretsFallbackOnlyForReleases(t *testing.T) {
	// A normal (non-release) app with neither a manifest nor a secrets.env, whose env has a
	// legitimate compose passthrough like ${TZ}. The fallback must NOT treat it as a missing
	// required secret, so start is not blocked. (Regression: previously it aborted the start.)
	normal := app.ArduinoApp{FullPath: paths.New(t.TempDir())}
	envs := helpers.EnvVars{"TZ": "${TZ}"}
	missing, err := applyReleaseSecrets(normal, envs)
	require.NoError(t, err)
	require.Empty(t, missing)

	// A frozen release with no readable manifest still gets the inference fallback, so a
	// scrubbed ${API_KEY} placeholder with no provided value is reported missing.
	release := app.ArduinoApp{
		FullPath:   paths.New(t.TempDir()),
		Descriptor: app.AppDescriptor{FrozenRelease: &app.FrozenReleaseInfo{Number: "1"}},
	}
	missing, err = applyReleaseSecrets(release, helpers.EnvVars{"API_KEY": "${API_KEY}"})
	require.NoError(t, err)
	require.Equal(t, []string{"API_KEY"}, missing)
}

func TestScrubComposeSecretValues(t *testing.T) {
	// A secret value embedded inside a larger value (connection string) that
	// scrubComposeSecretKeys does not catch, plus one whose value is a prefix of a longer
	// secret's value, to exercise the longest-first replacement.
	compose := "services:\n" +
		"  main:\n" +
		"    environment:\n" +
		"      DATABASE_URL: postgres://user:s3cr3t@db:5432/app\n" +
		"      TOKEN: abc123def\n"
	values := map[string][]string{
		"DB_PASSWORD": {"s3cr3t"},
		"API_KEY":     {"abc123def"},
		"SHORT":       {"abc123"}, // substring of API_KEY's value; must not corrupt it
	}
	out, err := scrubComposeSecretValues(compose, values)
	require.NoError(t, err)
	require.NotContains(t, out, "s3cr3t")
	require.NotContains(t, out, "abc123def")
	require.Contains(t, out, "postgres://user:${DB_PASSWORD}@db:5432/app")
	require.Contains(t, out, "TOKEN: ${API_KEY}")

	// Defense-in-depth: if a value literally cannot be removed the build fails rather than
	// leaking it. (Contrived: replacement is exhaustive, so this only guards regressions.)
	_, err = scrubComposeSecretValues("plain text", map[string][]string{"X": {""}})
	require.NoError(t, err) // empty value is a no-op
}

func TestScrubComposeSecretValuesMultipleValuesPerName(t *testing.T) {
	// Two bricks declare the same secret NAME with different values; both must be stripped,
	// even when one is only embedded (not on its own env line).
	compose := "services:\n" +
		"  main:\n" +
		"    environment:\n" +
		"      URL_A: http://user:alpha-secret@a\n" +
		"      URL_B: http://user:bravo-secret@b\n"
	values := map[string][]string{"API_KEY": {"alpha-secret", "bravo-secret"}}
	out, err := scrubComposeSecretValues(compose, values)
	require.NoError(t, err)
	require.NotContains(t, out, "alpha-secret")
	require.NotContains(t, out, "bravo-secret")
}

func TestScrubComposeSecretValuesShortValueNotCorrupting(t *testing.T) {
	// A short secret value ("8080") must NOT be blanket-replaced across the compose (which
	// would corrupt the port mapping). It is left for env-key scrubbing; here it appears only
	// in a structural position, so it must survive untouched and not trip the leak check.
	compose := "    ports:\n      - \"8080:8080\"\n"
	out, err := scrubComposeSecretValues(compose, map[string][]string{"PORTVAR": {"8080"}})
	require.NoError(t, err)
	require.Equal(t, compose, out) // unchanged
}

func TestTokenizeHostSpecificValues(t *testing.T) {
	// Simulates the flattened `docker compose config` output produced with the host-specific
	// vars set to sentinels, alongside an unrelated IP that is a superstring of a plausible
	// host IP. The sentinel must map to ${HOST_IP} while the unrelated IP is left intact.
	flattened := "services:\n" +
		"  main:\n" +
		"    environment:\n" +
		"      HOST_IP: " + sentinelHostIP + "\n" +
		"      PEER: 192.168.1.50\n" + // must NOT be tokenized
		"    volumes:\n" +
		"      - " + sentinelAppHome + ":/app\n" +
		"      - " + sentinelModelsDir + "/edge-impulse:/models\n"

	out := tokenizeHostSpecificValues(flattened, "/home/arduino/.models", "/home/arduino/apps/demo")

	require.Contains(t, out, "HOST_IP: ${HOST_IP}")
	require.Contains(t, out, "PEER: 192.168.1.50") // untouched: no over-match on the host IP
	require.Contains(t, out, "- ${APP_HOME}:/app")
	require.Contains(t, out, "- ${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}/edge-impulse:/models")
	require.NotContains(t, out, sentinelHostIP)
	require.NotContains(t, out, sentinelAppHome)
	require.NotContains(t, out, sentinelModelsDir)
}

func TestTokenizeHostSpecificEnv(t *testing.T) {
	// Reproduces host-specific values baked as literals by provisioning: an `environment:`
	// block (as emitted by `docker compose config`) plus bind-mount source/target paths for
	// the model directories. All must become ${KEY} placeholders while pinned, non-host
	// values (image tags, BOARD_NAME, BIND_ADDRESS) are left untouched.
	flattened := "services:\n" +
		"  runner:\n" +
		"    image: ghcr.io/arduino/app-bricks/ei-models-runner:0.11.2\n" +
		"    environment:\n" +
		"      APP_HOME: ${APP_HOME}\n" +
		"      BIND_ADDRESS: 127.0.0.1\n" +
		"      BOARD_NAME: unoq\n" +
		"      CUSTOM_MODEL_PATH: /home/arduino/.arduino-bricks/ei-models\n" +
		"      HOST_IP: 192.168.1.61\n" +
		"      MODELS_PATH: /var/lib/arduino-app-cli/models\n" +
		"    volumes:\n" +
		"      - type: bind\n" +
		"        source: /home/arduino/.arduino-bricks/ei-models\n" +
		"        target: /home/arduino/.arduino-bricks/ei-models\n" +
		"      - type: bind\n" +
		"        source: /var/lib/arduino-app-cli/models/edge-impulse\n" +
		"        target: /var/lib/arduino-app-cli/models/edge-impulse\n"

	envs := helpers.EnvVars{
		"HOST_IP":           "192.168.1.61",
		"MODELS_PATH":       "/var/lib/arduino-app-cli/models",
		"CUSTOM_MODEL_PATH": "/home/arduino/.arduino-bricks/ei-models",
	}

	out := tokenizeHostSpecificEnv(flattened, envs)

	// Env blocks neutralized by key.
	require.Contains(t, out, "HOST_IP: ${HOST_IP}")
	require.Contains(t, out, "MODELS_PATH: ${MODELS_PATH}")
	require.Contains(t, out, "CUSTOM_MODEL_PATH: ${CUSTOM_MODEL_PATH}")
	// Bind-mount paths neutralized by value.
	require.Contains(t, out, "source: ${CUSTOM_MODEL_PATH}")
	require.Contains(t, out, "source: ${MODELS_PATH}/edge-impulse")
	require.Contains(t, out, "target: ${MODELS_PATH}/edge-impulse")
	// No host-specific literal survives anywhere.
	require.NotContains(t, out, "192.168.1.61")
	require.NotContains(t, out, "/home/arduino/.arduino-bricks/ei-models")
	require.NotContains(t, out, "/var/lib/arduino-app-cli/models")
	// Pinned, non-host values are preserved.
	require.Contains(t, out, "image: ghcr.io/arduino/app-bricks/ei-models-runner:0.11.2")
	require.Contains(t, out, "BOARD_NAME: unoq")
	require.Contains(t, out, "BIND_ADDRESS: 127.0.0.1")
	require.Contains(t, out, "APP_HOME: ${APP_HOME}")
}

func TestApplyHostEnvDefaults(t *testing.T) {
	// The frozen compose carries the portable ${KEY} placeholders produced at build time,
	// both in env blocks and bind-mount paths. Localization must turn the stable paths into
	// literals and HOST_IP into a ${HOST_IP:-<ip>} default so offline commands never see a
	// blank value while `app start` still overrides it.
	compose := "services:\n" +
		"  main:\n" +
		"    environment:\n" +
		"      CUSTOM_MODEL_PATH: ${CUSTOM_MODEL_PATH}\n" +
		"      HOST_IP: ${HOST_IP}\n" +
		"      MODELS_PATH: ${MODELS_PATH}\n" +
		"    volumes:\n" +
		"      - source: ${CUSTOM_MODEL_PATH}\n" +
		"      - source: ${MODELS_PATH}/edge-impulse\n"

	envs := helpers.EnvVars{
		"HOST_IP":           "10.0.0.7",
		"MODELS_PATH":       "/var/lib/arduino-app-cli/models",
		"CUSTOM_MODEL_PATH": "/home/arduino/.arduino-bricks/ei-models",
	}

	out := applyHostEnvDefaults(compose, envs)

	require.Contains(t, out, "HOST_IP: ${HOST_IP:-10.0.0.7}")
	require.Contains(t, out, "MODELS_PATH: /var/lib/arduino-app-cli/models")
	require.Contains(t, out, "CUSTOM_MODEL_PATH: /home/arduino/.arduino-bricks/ei-models")
	require.Contains(t, out, "source: /home/arduino/.arduino-bricks/ei-models")
	require.Contains(t, out, "source: /var/lib/arduino-app-cli/models/edge-impulse")
	// No bare, unresolved placeholder remains (which is what makes docker compose warn).
	require.NotContains(t, out, "${MODELS_PATH}")
	require.NotContains(t, out, "${CUSTOM_MODEL_PATH}")
	require.NotContains(t, out, "HOST_IP: ${HOST_IP}\n")
}

// A missing key must be left as a bare placeholder rather than substituted with a blank.
func TestApplyHostEnvDefaultsMissingKey(t *testing.T) {
	compose := "      HOST_IP: ${HOST_IP}\n      MODELS_PATH: ${MODELS_PATH}\n"
	out := applyHostEnvDefaults(compose, helpers.EnvVars{"MODELS_PATH": "/m"})
	require.Contains(t, out, "HOST_IP: ${HOST_IP}") // untouched: no value to default to
	require.Contains(t, out, "MODELS_PATH: /m")
}

func TestHasProvisionedVenv(t *testing.T) {
	cache := paths.New(t.TempDir())
	require.False(t, hasProvisionedVenv(cache))
	writeFile(t, cache.Join("venv", "pyvenv.cfg"), "home = /usr\n")
	require.True(t, hasProvisionedVenv(cache))
}

func requireFileContent(t *testing.T, p *paths.Path, want string) {
	t.Helper()
	got, err := os.ReadFile(p.String())
	require.NoError(t, err, "reading %s", p)
	require.Equal(t, want, string(got))
}
