// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"github.com/gofrs/flock"
	"github.com/shirou/gopsutil/v4/disk"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

const (
	brickConfigFileName = "brick_config.yaml"
	installLockFileName = ".lock"
	// Temporary folders start with a dot, which no namespace can, so they never clash with a brick.
	installTempPrefix   = ".install-"
	uninstallTempPrefix = ".uninstall-"
)

var (
	ErrBrickNotFound       = errors.New("brick not found in the bricks index")
	ErrReleaseNotFound     = errors.New("brick version not found in the bricks index")
	ErrInsufficientStorage = errors.New("not enough disk space to install the brick")

	// Namespaces and names become folder names, so they are limited to a safe set of characters.
	validFolderName = regexp.MustCompile(`^[a-zA-Z0-9_-][a-zA-Z0-9_.-]*$`)
)

// ParseBrickReference splits a reference in the form ID[@VERSION]. The version is empty
// when the reference does not have one.
func ParseBrickReference(ref string) (id string, version string, err error) {
	id, version, hasVersion := strings.Cut(ref, "@")
	if _, _, err := bricksindex.ParseBrickID(id); err != nil {
		return "", "", fmt.Errorf("invalid brick reference %q: expected NAMESPACE:NAME[@VERSION]", ref)
	}
	if hasVersion && version == "" {
		return "", "", fmt.Errorf("invalid brick reference %q: missing version after '@'", ref)
	}
	return id, version, nil
}

// FindRelease returns the brick with the given id and its release with the given version,
// or its latest release if version is empty.
func (idx *Index) FindRelease(id, version string) (*Brick, *Release, error) {
	for i := range idx.Bricks {
		brick := &idx.Bricks[i]
		if brick.ID != id {
			continue
		}
		if version == "" {
			version = brick.Latest
		}
		for j := range brick.Releases {
			if brick.Releases[j].Version == version {
				return brick, &brick.Releases[j], nil
			}
		}
		return nil, nil, fmt.Errorf("%w: %s@%s", ErrReleaseNotFound, id, version)
	}
	return nil, nil, fmt.Errorf("%w: %s", ErrBrickNotFound, id)
}

// splitBrickID returns the namespace and the name of the brick, checking that both can be used as folder names.
func splitBrickID(id string) (namespace, name string, err error) {
	namespace, name, err = bricksindex.ParseBrickID(id)
	if err != nil || !validFolderName.MatchString(namespace) || !validFolderName.MatchString(name) {
		return "", "", fmt.Errorf("brick id %q contains characters that are not allowed", id)
	}
	return namespace, name, nil
}

// InstallPath returns the folder where the given version of the brick is installed in bricksDir.
func InstallPath(bricksDir *paths.Path, id, version string) (*paths.Path, error) {
	namespace, name, err := splitBrickID(id)
	if err != nil {
		return nil, err
	}
	// An empty string is accepted by the parser.
	if version == "" {
		return nil, fmt.Errorf("brick %q has a release without version", id)
	}
	if _, err := semver.Parse(version); err != nil {
		return nil, fmt.Errorf("brick %q has an invalid version %q: %w", id, version, err)
	}
	return bricksDir.Join(namespace, name+"@"+version), nil
}

// lockBricksDir prevents installs and uninstalls from running at the same time.
func lockBricksDir(ctx context.Context, bricksDir *paths.Path) (unlock func(), err error) {
	lock := flock.New(bricksDir.Join(installLockFileName).String())
	if _, err := lock.TryLockContext(ctx, 100*time.Millisecond); err != nil {
		return nil, fmt.Errorf("locking bricks directory: %w", err)
	}
	return func() { _ = lock.Unlock() }, nil
}

type InstallResult struct {
	Path *paths.Path
	// AlreadyInstalled is true when the release was found installed and nothing was done.
	AlreadyInstalled bool
}

// Install downloads the release of the brick and installs it in bricksDir. Releases are
// never replaced: each version is installed in its own folder.
func Install(ctx context.Context, client *http.Client, bricksDir *paths.Path, brick *Brick, release *Release, userAgent string) (InstallResult, error) {
	installPath, err := InstallPath(bricksDir, brick.ID, release.Version)
	if err != nil {
		return InstallResult{}, err
	}
	expectedHash, err := hex.DecodeString(release.Hash)
	if err != nil || len(expectedHash) != sha256.Size {
		return InstallResult{}, fmt.Errorf("brick %s@%s: invalid SHA-256 hash %q", brick.ID, release.Version, release.Hash)
	}
	if release.Size <= 0 {
		return InstallResult{}, fmt.Errorf("brick %s@%s: invalid size %d", brick.ID, release.Version, release.Size)
	}
	if release.URL == "" {
		return InstallResult{}, fmt.Errorf("brick %s@%s: missing download url", brick.ID, release.Version)
	}

	if err := bricksDir.MkdirAll(); err != nil {
		return InstallResult{}, fmt.Errorf("creating bricks directory: %w", err)
	}
	unlock, err := lockBricksDir(ctx, bricksDir)
	if err != nil {
		return InstallResult{}, err
	}
	defer unlock()

	if installPath.Exist() {
		return InstallResult{Path: installPath, AlreadyInstalled: true}, nil
	}

	// Created in bricksDir so that the final rename does not cross filesystems.
	tempDir, err := paths.MkTempDir(bricksDir.String(), installTempPrefix)
	if err != nil {
		return InstallResult{}, fmt.Errorf("creating temporary directory: %w", err)
	}
	defer func() { _ = tempDir.RemoveAll() }()

	archivePath := tempDir.Join("archive.zip")
	if err := checkFreeSpace(bricksDir, uint64(release.Size)); err != nil {
		return InstallResult{}, fmt.Errorf("downloading brick %s@%s: %w", brick.ID, release.Version, err)
	}
	if err := download(ctx, client, release.URL, archivePath, release.Size, expectedHash, userAgent); err != nil {
		return InstallResult{}, fmt.Errorf("downloading brick %s@%s: %w", brick.ID, release.Version, err)
	}

	extractDir := tempDir.Join("extract")
	if err := extractDir.Mkdir(); err != nil {
		return InstallResult{}, fmt.Errorf("creating temporary directory: %w", err)
	}
	rootName, err := extractZip(archivePath, extractDir)
	if err != nil {
		return InstallResult{}, fmt.Errorf("extracting brick %s@%s: %w", brick.ID, release.Version, err)
	}
	brickDir := extractDir.Join(rootName)
	if err := checkBrickConfig(brickDir, brick.ID); err != nil {
		return InstallResult{}, fmt.Errorf("brick %s@%s: %w", brick.ID, release.Version, err)
	}

	if err := installPath.Parent().MkdirAll(); err != nil {
		return InstallResult{}, fmt.Errorf("creating brick directory: %w", err)
	}
	fatomic.Sync()
	if err := brickDir.Rename(installPath); err != nil {
		return InstallResult{}, fmt.Errorf("installing brick %s@%s: %w", brick.ID, release.Version, err)
	}
	fatomic.Sync()

	return InstallResult{Path: installPath}, nil
}

// checkFreeSpace returns ErrInsufficientStorage if the filesystem of dir has less than required bytes free.
func checkFreeSpace(dir *paths.Path, required uint64) error {
	usage, err := disk.Usage(dir.String())
	if err != nil {
		return fmt.Errorf("reading free disk space: %w", err)
	}
	if required > usage.Free {
		return fmt.Errorf("%w: %d bytes needed, %d bytes free", ErrInsufficientStorage, required, usage.Free)
	}
	return nil
}

func download(ctx context.Context, client *http.Client, url string, dest *paths.Path, size int64, expectedHash []byte, userAgent string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s from %s", resp.Status, url)
	}

	file, err := dest.Create()
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	// One byte more than expected, to detect a bigger archive without reading all of it.
	written, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(resp.Body, size+1))
	if err != nil {
		return err
	}
	if written != size {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d", size, written)
	}
	if actual := hasher.Sum(nil); !bytes.Equal(actual, expectedHash) {
		return fmt.Errorf("SHA-256 mismatch: expected %x, got %x", expectedHash, actual)
	}
	return file.Close()
}

// extractZip extracts the archive into destDir and returns the name of its root folder.
// The archive must contain a single root folder, ignoring the __MACOSX metadata folder.
func extractZip(archive *paths.Path, destDir *paths.Path) (string, error) {
	reader, err := zip.OpenReader(archive.String())
	if err != nil {
		return "", fmt.Errorf("invalid archive: %w", err)
	}
	defer reader.Close()

	// The sizes declared by the archive can be trusted: its hash has already been verified.
	var extractedSize uint64
	for _, f := range reader.File {
		extractedSize += f.UncompressedSize64
	}
	if err := checkFreeSpace(destDir, extractedSize); err != nil {
		return "", err
	}
	// Writing through a root prevents any entry from escaping destDir.
	destRoot, err := os.OpenRoot(destDir.String())
	if err != nil {
		return "", err
	}
	defer destRoot.Close()

	var rootName string
	for _, f := range reader.File {
		name := path.Clean(strings.ReplaceAll(f.Name, `\`, "/"))
		root, entry, _ := strings.Cut(name, "/")
		if root == "__MACOSX" {
			continue
		}
		if path.IsAbs(name) || root == "" || root == "." || root == ".." {
			return "", fmt.Errorf("invalid entry %s", f.Name)
		}
		if rootName == "" {
			rootName = root
		}
		if root != rootName {
			return "", errors.New("the archive must contain a single root folder")
		}

		mode := f.Mode()
		switch {
		case mode.IsDir():
			err = destRoot.MkdirAll(name, 0o755)
		case mode.IsRegular():
			if entry == "" {
				return "", errors.New("the archive must contain a single root folder")
			}
			err = extractFile(destRoot, f, name)
		default:
			return "", fmt.Errorf("unsupported entry %s: only files and folders are allowed", f.Name)
		}
		if err != nil {
			return "", fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}
	if rootName == "" {
		return "", errors.New("the archive is empty")
	}
	return rootName, nil
}

func extractFile(destRoot *os.Root, f *zip.File, name string) error {
	if err := destRoot.MkdirAll(path.Dir(name), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := destRoot.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, f.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

// checkBrickConfig verifies that brickDir contains a brick_config.yaml describing the brick with the given id.
func checkBrickConfig(brickDir *paths.Path, id string) error {
	data, err := brickDir.Join(brickConfigFileName).ReadFile()
	if err != nil {
		return fmt.Errorf("the archive does not contain a valid %s: %w", brickConfigFileName, err)
	}
	var config struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("invalid %s: %w", brickConfigFileName, err)
	}
	if config.ID != id {
		return fmt.Errorf("%s declares id %q instead of %q", brickConfigFileName, config.ID, id)
	}
	return nil
}
