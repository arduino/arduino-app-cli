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
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/gofrs/flock"
	"github.com/shirou/gopsutil/v4/disk"
	"go.bug.st/downloader/v3"
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

// FindBrick returns the brick with the given id.
func (idx *RemoteIndex) FindBrick(id string) (*RemoteBrick, error) {
	for i := range idx.Bricks {
		if idx.Bricks[i].ID == id {
			return &idx.Bricks[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrBrickNotFound, id)
}

// FindRelease returns the release with the given version or, if version is nil,
// the latest release that supports the board of platform.
func (b *RemoteBrick) FindRelease(version *semver.Version, platform platform.Platform) (*RemoteRelease, error) {
	if version == nil {
		if latest := b.Latest(platform); latest != nil {
			return latest, nil
		}
		return nil, fmt.Errorf("%w: no release of %s supports %s", ErrUnsupportedBoard, b.ID, platform.BoardName)
	}
	for i := range b.Releases {
		r := &b.Releases[i]
		if !r.Version.Equal(version) {
			continue
		}
		if !r.SupportsBoard(platform) {
			return nil, fmt.Errorf("%w: %s@%s does not support %s", ErrUnsupportedBoard, b.ID, version, platform.BoardName)
		}
		return r, nil
	}
	return nil, fmt.Errorf("%w: %s@%s", ErrReleaseNotFound, b.ID, version)
}

// Latest returns the release with the highest version that supports the board of platform,
// or nil if there is none.
func (b *RemoteBrick) Latest(platform platform.Platform) *RemoteRelease {
	var latest *RemoteRelease
	for i := range b.Releases {
		r := &b.Releases[i]
		if r.SupportsBoard(platform) && (latest == nil || r.Version.GreaterThan(latest.Version)) {
			latest = r
		}
	}
	return latest
}

// SupportedReleases returns the releases that support the board of platform, sorted by version.
func (b *RemoteBrick) SupportedReleases(platform platform.Platform) []*RemoteRelease {
	var res []*RemoteRelease
	for i := range b.Releases {
		if b.Releases[i].SupportsBoard(platform) {
			res = append(res, &b.Releases[i])
		}
	}
	slices.SortFunc(res, func(a, b *RemoteRelease) int { return a.Version.CompareTo(b.Version) })
	return res
}

// splitBrickID returns the namespace and the name of the brick, checking that both can be used as folder names.
func splitBrickID(id string) (namespace, name string, err error) {
	namespace, name, err = ParseBrickID(id)
	if err != nil || !validBrickIDPart.MatchString(namespace) || !validBrickIDPart.MatchString(name) {
		return "", "", fmt.Errorf("invalid brick id %q: expected NAMESPACE:NAME, each made of lowercase letters, digits and underscores, starting with a letter", id)
	}
	return namespace, name, nil
}

// installPath returns the folder where the given version of the brick is installed in bricksDir.
// The id must be valid.
func installPath(bricksDir *paths.Path, id string, version *semver.Version) *paths.Path {
	namespace, name, _ := ParseBrickID(id)
	return bricksDir.Join(namespace, name, version.String())
}

// lockBricksDir prevents installs and uninstalls from running at the same time. If another
// operation holds the lock, onWait is called. Once locked, it removes the temporary folders
// left by interrupted operations.
func lockBricksDir(ctx context.Context, bricksDir *paths.Path, onWait func()) (unlock func(), err error) {
	lock := flock.New(bricksDir.Join(installLockFileName).String())
	locked, err := lock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("locking bricks directory: %w", err)
	}
	if !locked {
		if onWait != nil {
			onWait()
		}
		if _, err := lock.TryLockContext(ctx, 100*time.Millisecond); err != nil {
			return nil, fmt.Errorf("locking bricks directory: %w", err)
		}
	}
	removeTempDirs(bricksDir)
	return func() { _ = lock.Unlock() }, nil
}

// removeTempDirs removes the temporary folders of installs and uninstalls. It must be called while
// holding the lock, so that no operation using them is running.
func removeTempDirs(bricksDir *paths.Path) {
	entries, err := bricksDir.ReadDir()
	if err != nil {
		slog.Warn("cannot read bricks directory", "path", bricksDir, "error", err)
		return
	}
	for _, entry := range entries {
		name := entry.Base()
		if !strings.HasPrefix(name, installTempPrefix) && !strings.HasPrefix(name, uninstallTempPrefix) {
			continue
		}
		if err := entry.RemoveAll(); err != nil {
			slog.Warn("cannot remove temporary folder", "path", entry, "error", err)
		}
	}
}

type InstallResult struct {
	Path *paths.Path
	// AlreadyInstalled is true when the release was found installed and nothing was done.
	AlreadyInstalled bool
}

// Install downloads the release of the brick and installs it in bricksDir. If prepare is not nil,
// required containers are installed. The brick and the release must come from a validated index.
func Install(ctx context.Context, config downloader.Config, bricksDir *paths.Path, brick *RemoteBrick, release *RemoteRelease, platform platform.Platform, progress func(downloaded, total int64), onWait func(), prepare func(Brick) error) (InstallResult, error) {
	ref := brick.ID + "@" + release.Version.String()
	expectedHash, err := hex.DecodeString(release.Hash)
	if err != nil {
		return InstallResult{}, err
	}
	brickPath := installPath(bricksDir, brick.ID, release.Version)
	if err := bricksDir.MkdirAll(); err != nil {
		return InstallResult{}, fmt.Errorf("creating bricks directory: %w", err)
	}
	unlock, err := lockBricksDir(ctx, bricksDir, onWait)
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
	if err := checkFreeSpace(bricksDir, uint64(release.Size)); err != nil { // nolint:gosec // the index validation checks that the size is positive
		return InstallResult{}, fmt.Errorf("downloading brick %s: %w", ref, err)
	}
	var downloadProgress func(int64)
	if progress != nil {
		downloadProgress = func(downloaded int64) { progress(downloaded, release.Size) }
	}
	if err := downloadArchive(ctx, config, release, archivePath, expectedHash, downloadProgress); err != nil {
		return InstallResult{}, fmt.Errorf("downloading brick %s: %w", ref, err)
	}

	extractDir := tempDir.Join("extract")
	if err := extractDir.Mkdir(); err != nil {
		return InstallResult{}, fmt.Errorf("creating temporary directory: %w", err)
	}
	if err := extractZip(archivePath, extractDir); err != nil {
		return InstallResult{}, fmt.Errorf("extracting brick %s: %w", ref, err)
	}
	brickDir, err := releaseRoot(extractDir)
	if err != nil {
		return InstallResult{}, fmt.Errorf("brick %s: %w", ref, err)
	}
	if prepare != nil {
		namespace, brickName, _ := ParseBrickID(brick.ID)
		extracted, err := loadInstalledBrick(platform, brickDir, namespace, brickName)
		if err != nil {
			return InstallResult{}, fmt.Errorf("brick %s: %w", ref, err)
		}
		extracted.Version = release.Version.String()
		if err := prepare(extracted); err != nil {
			return InstallResult{}, fmt.Errorf("installing brick %s: %w", ref, err)
		}
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

// downloadArchive downloads the archive of the release into dest and checks its size and hash.
func downloadArchive(ctx context.Context, config downloader.Config, release *RemoteRelease, dest *paths.Path, expectedHash []byte, progress func(int64)) error {
	if err := downloadFile(ctx, config, release.URL, dest.String(), release.Size, release.Size, progress); err != nil {
		return err
	}
	file, err := dest.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return err
	}
	if size != release.Size {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d", release.Size, size)
	}
	if actual := hasher.Sum(nil); !bytes.Equal(actual, expectedHash) {
		return fmt.Errorf("SHA-256 mismatch: expected %x, got %x", expectedHash, actual)
	}
	return nil
}

// extractZip extracts the archive into destDir. The archive is trusted, as the registry checks
// it and its hash has been verified.
func extractZip(archive *paths.Path, destDir *paths.Path) error {
	reader, err := zip.OpenReader(archive.String())
	if err != nil {
		return fmt.Errorf("invalid archive: %w", err)
	}
	defer reader.Close()

	var extractedSize uint64
	for _, f := range reader.File {
		extractedSize += f.UncompressedSize64
	}
	if err := checkFreeSpace(destDir, extractedSize); err != nil {
		return err
	}
	// Writing through a root prevents any entry from escaping destDir.
	destRoot, err := os.OpenRoot(destDir.String())
	if err != nil {
		return err
	}
	defer destRoot.Close()

	for _, f := range reader.File {
		mode := f.Mode()
		switch {
		case mode.IsDir():
			err = destRoot.MkdirAll(f.Name, 0o755)
		case mode.IsRegular():
			err = extractFile(destRoot, f)
		default:
			return fmt.Errorf("unsupported entry %s: only files and folders are allowed", f.Name)
		}
		if err != nil {
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}
	return nil
}

// releaseRoot returns the folder of the extracted release that contains src/: the archive
// has it at its root (as git archive makes it) or inside a single root folder.
func releaseRoot(extractDir *paths.Path) (*paths.Path, error) {
	if extractDir.Join(releaseSrcDir).IsDir() {
		return extractDir, nil
	}
	entries, err := extractDir.ReadDir()
	if err != nil {
		return nil, err
	}
	if len(entries) == 1 && entries[0].Join(releaseSrcDir).IsDir() {
		return entries[0], nil
	}
	return nil, fmt.Errorf("the archive does not contain a %s folder, at its root or inside a single root folder", releaseSrcDir)
}

func extractFile(destRoot *os.Root, f *zip.File) error {
	if err := destRoot.MkdirAll(path.Dir(f.Name), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := destRoot.OpenFile(f.Name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, f.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil { // nolint:gosec // the archive is trusted: the registry checks it and its hash is verified
		return err
	}
	return dst.Close()
}
