// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"github.com/gofrs/flock"
	"github.com/shirou/gopsutil/v4/disk"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/platform"
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
	ErrUnsupportedBoard    = errors.New("brick not supported by this board")
	ErrInsufficientStorage = errors.New("not enough disk space to install the brick")
)

// ParseBrickReference splits a reference in the form ID[@VERSION]. The version is nil
// when the reference does not have one.
func ParseBrickReference(ref string) (id string, version *semver.Version, err error) {
	id, versionString, hasVersion := strings.Cut(ref, "@")
	if _, _, err := splitBrickID(id); err != nil {
		return "", nil, fmt.Errorf("invalid brick reference %q: expected NAMESPACE:NAME[@VERSION]", ref)
	}
	if !hasVersion {
		return id, nil, nil
	}
	// semver.Parse accepts an empty string.
	if versionString == "" {
		return "", nil, fmt.Errorf("invalid brick reference %q: missing version after '@'", ref)
	}
	version, err = semver.Parse(versionString)
	if err != nil {
		return "", nil, fmt.Errorf("invalid brick reference %q: %w", ref, err)
	}
	return id, version, nil
}

// FindRelease returns the release of the brick with the given version or, if version is nil,
// the latest release that supports the board of platform.
func (idx *RemoteIndex) FindRelease(id string, version *semver.Version, platform platform.Platform) (*RemoteBrick, error) {
	releases := idx.releases(id)
	if len(releases) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrBrickNotFound, id)
	}
	if version == nil {
		if latest := idx.Latest(id, platform); latest != nil {
			return latest, nil
		}
		return nil, fmt.Errorf("%w: no release of %s supports %s", ErrUnsupportedBoard, id, platform.BoardName)
	}
	for _, r := range releases {
		if !r.Version.Equal(version) {
			continue
		}
		if !r.SupportsBoard(platform) {
			return nil, fmt.Errorf("%w: %s@%s does not support %s", ErrUnsupportedBoard, id, version, platform.BoardName)
		}
		return r, nil
	}
	return nil, fmt.Errorf("%w: %s@%s", ErrReleaseNotFound, id, version)
}

// Latest returns the release with the highest version of the brick that supports the board of
// platform, or nil if there is none.
func (idx *RemoteIndex) Latest(id string, platform platform.Platform) *RemoteBrick {
	var latest *RemoteBrick
	for _, r := range idx.releases(id) {
		if r.SupportsBoard(platform) && (latest == nil || r.Version.GreaterThan(latest.Version)) {
			latest = r
		}
	}
	return latest
}

// Releases returns the releases of the brick that support the board of platform, sorted by version.
func (idx *RemoteIndex) Releases(id string, platform platform.Platform) []*RemoteBrick {
	res := slices.DeleteFunc(idx.releases(id), func(r *RemoteBrick) bool { return !r.SupportsBoard(platform) })
	slices.SortFunc(res, func(a, b *RemoteBrick) int { return a.Version.CompareTo(b.Version) })
	return res
}

// releases returns all the releases of the brick.
func (idx *RemoteIndex) releases(id string) []*RemoteBrick {
	var res []*RemoteBrick
	for i := range idx.Bricks {
		if idx.Bricks[i].ID == id {
			res = append(res, &idx.Bricks[i])
		}
	}
	return res
}

// splitBrickID returns the namespace and the name of the brick, checking that both can be used as folder names.
func splitBrickID(id string) (namespace, name string, err error) {
	namespace, name, err = ParseBrickID(id)
	if err != nil || !validBrickIDPart.MatchString(namespace) || !validBrickIDPart.MatchString(name) {
		return "", "", fmt.Errorf("invalid brick id %q: expected NAMESPACE:NAME, each made of lowercase letters, digits and underscores", id)
	}
	return namespace, name, nil
}

// installPath returns the folder where the given version of the brick is installed in bricksDir.
// The id must be valid.
func installPath(bricksDir *paths.Path, id string, version *semver.Version) *paths.Path {
	namespace, name, _ := ParseBrickID(id)
	return bricksDir.Join(namespace, name, version.String())
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
func Install(ctx context.Context, client *http.Client, bricksDir *paths.Path, brick *RemoteBrick, platform platform.Platform, userAgent string) (InstallResult, error) {
	// The release usually comes from a validated index: this protects the paths built from it.
	if err := brick.validate(); err != nil {
		return InstallResult{}, err
	}
	ref := brick.ID + "@" + brick.Version.String()
	if !brick.SupportsBoard(platform) {
		return InstallResult{}, fmt.Errorf("%w: %s does not support %s", ErrUnsupportedBoard, ref, platform.BoardName)
	}
	expectedHash, err := hex.DecodeString(brick.Hash)
	if err != nil {
		return InstallResult{}, err
	}
	brickPath := installPath(bricksDir, brick.ID, brick.Version)
	if err := bricksDir.MkdirAll(); err != nil {
		return InstallResult{}, fmt.Errorf("creating bricks directory: %w", err)
	}
	unlock, err := lockBricksDir(ctx, bricksDir)
	if err != nil {
		return InstallResult{}, err
	}
	defer unlock()

	if brickPath.Exist() {
		return InstallResult{Path: brickPath, AlreadyInstalled: true}, nil
	}

	// Created in bricksDir so that the final rename does not cross filesystems.
	tempDir, err := paths.MkTempDir(bricksDir.String(), installTempPrefix)
	if err != nil {
		return InstallResult{}, fmt.Errorf("creating temporary directory: %w", err)
	}
	defer func() { _ = tempDir.RemoveAll() }()

	archivePath := tempDir.Join("archive.zip")
	if err := checkFreeSpace(bricksDir, uint64(brick.Size)); err != nil { // nolint:gosec // validate checks that the size is positive
		return InstallResult{}, fmt.Errorf("downloading brick %s: %w", ref, err)
	}
	if err := download(ctx, client, brick.URL, archivePath, brick.Size, expectedHash, userAgent); err != nil {
		return InstallResult{}, fmt.Errorf("downloading brick %s: %w", ref, err)
	}

	extractDir := tempDir.Join("extract")
	if err := extractDir.Mkdir(); err != nil {
		return InstallResult{}, fmt.Errorf("creating temporary directory: %w", err)
	}
	rootName, err := extractZip(archivePath, extractDir)
	if err != nil {
		return InstallResult{}, fmt.Errorf("extracting brick %s: %w", ref, err)
	}
	brickDir := extractDir.Join(rootName)
	if err := checkBrickConfig(brickDir, brick.ID); err != nil {
		return InstallResult{}, fmt.Errorf("brick %s: %w", ref, err)
	}

	if err := brickPath.Parent().MkdirAll(); err != nil {
		return InstallResult{}, fmt.Errorf("creating brick directory: %w", err)
	}
	fatomic.Sync()
	if err := brickDir.Rename(brickPath); err != nil {
		return InstallResult{}, fmt.Errorf("installing brick %s: %w", ref, err)
	}
	fatomic.Sync()

	return InstallResult{Path: brickPath}, nil
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
	if f.UncompressedSize64 >= math.MaxInt64 {
		return fmt.Errorf("invalid size %d", f.UncompressedSize64)
	}
	size := int64(f.UncompressedSize64)
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
	defer dst.Close()
	// The declared size has been accounted for by the free space check: never write more than that.
	written, err := io.Copy(dst, io.LimitReader(src, size+1))
	if err != nil {
		return err
	}
	if written > size {
		return fmt.Errorf("file larger than its declared size %d", size)
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
