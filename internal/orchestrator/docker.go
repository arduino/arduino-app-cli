// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package orchestrator

import (
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	semver "go.bug.st/relaxed-semver"
	"golang.org/x/sys/unix"
)

// Returns the total free disk space in bytes, in the partition where docker stores images.
func GetDockerFreeSpace() (uint64, error) {
	var stat unix.Statfs_t

	err := unix.Statfs("/var/lib/docker", &stat)
	if err != nil {
		return 0, err
	}

	return stat.Bavail * uint64(stat.Bsize), nil //nolint:gosec
}

// Returns the highest version of a given docker image, from the input list, matching the targetImage.
func GetHighestVersion(targetImage string, existingImages []string) string {
	targetBase, _ := parseDockerImage(targetImage)

	var highestVer *semver.Version
	var highestImg = ""

	for _, img := range existingImages {
		name, version := parseDockerImage(img)

		if name != targetBase {
			continue
		}

		v, err := semver.Parse(version)
		if err != nil {
			return ""
		}

		if highestVer == nil || !v.LessThan(highestVer) {
			highestVer = v
			highestImg = img
		}
	}

	// If no matching image is found, an empty string is returned
	return highestImg
}

// Splits a docker image in the name and tag/version parts.
func parseDockerImage(image string) (name string, version string) {
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		return image[:idx], image[idx+1:]
	}
	if idx := strings.LastIndex(image, ":"); idx != -1 {
		return image[:idx], image[idx+1:]
	}
	return image, ""
}

// Returns the number of bytes that would be downloaded when pulling the new docker image while the old one is
// already present locally. It accounts for image layers that are already present locally.
func GetBytesToDownload(localRefStr string, remoteRefStr string, stdout io.Writer) (int64, error) {
	localLayers, err := getImageLayers(localRefStr)
	if err != nil {
		return 0, err
	}

	remoteLayers, err := getImageLayers(remoteRefStr)
	if err != nil {
		return 0, err
	}

	localDigests := map[string]struct{}{}
	for _, l := range localLayers {
		h, err := l.Digest()
		if err != nil {
			return 0, fmt.Errorf("error getting layer hash for %s: %w", localRefStr, err)
		}
		localDigests[h.String()] = struct{}{}
	}

	var downloadBytes int64
	for i, l := range remoteLayers {
		h, err := l.Digest()
		if err != nil {
			return 0, fmt.Errorf("error getting layer hash for %s: %w", remoteRefStr, err)
		}

		size, err := l.Size()
		if err != nil {
			return 0, fmt.Errorf("error getting size of layer %s: %w", h.String(), err)
		}

		if _, ok := localDigests[h.String()]; ok {
			fmt.Fprintf(stdout, "[%02d] PRESENT  %s (%d bytes)\n", i, h, size)
			continue
		}

		fmt.Fprintf(stdout, "[%02d] MISSING  %s (%d bytes)\n", i, h, size)
		downloadBytes += size
	}

	// TODO: After review, remove the logging code from this function.
	fmt.Fprintf(stdout, "Total bytes %d to download for %s\n", downloadBytes, remoteRefStr)
	return downloadBytes, nil
}

func getImageLayers(imageName string) ([]v1.Layer, error) {
	if len(imageName) == 0 {
		// If the imageName is empty, return an empty list of layers.
		return nil, nil
	}

	imageRef, err := name.ParseReference(imageName)
	if err != nil {
		return nil, fmt.Errorf("error parsing image name %s: %w", imageName, err)
	}

	dockerImage, err := remote.Image(imageRef)
	if err != nil {
		return nil, fmt.Errorf("error fetching manifest for %s: %w", imageName, err)
	}

	imageLayers, err := dockerImage.Layers()
	if err != nil {
		return nil, fmt.Errorf("error getting layers for %s: %w", imageName, err)
	}

	return imageLayers, nil
}
