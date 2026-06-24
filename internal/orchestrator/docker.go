// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/shirou/gopsutil/v4/disk"
	semver "go.bug.st/relaxed-semver"
)

// Returns the total free disk space in bytes, in the partition where docker stores images.
func GetDockerFreeSpace() (uint64, error) {
	usage, err := disk.Usage("/var/lib/docker")
	if err != nil {
		return 0, err
	}

	return usage.Free, nil
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
			// Skip any invalid semver tags like "latest".
			continue
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

// imageDisplayName returns a short, human-friendly name for a docker image
// reference, e.g. "python-apps-base" for
// "ghcr.io/arduino/app-bricks/python-apps-base:0.11.0rc6".
func imageDisplayName(image string) string {
	name, _ := parseDockerImage(image)
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		return name[idx+1:]
	}
	return name
}

// Returns the number of bytes that would be downloaded when pulling the new docker image while the old one is
// already present locally. It accounts for image layers that are already present locally.
func GetBytesToDownload(localRefStr string, remoteRefStr string) (int64, error) {
	layers, err := missingLayers(localRefStr, remoteRefStr)
	if err != nil {
		return 0, err
	}

	var downloadBytes int64
	for _, l := range layers {
		downloadBytes += l.Size
	}

	slog.Debug("docker image bytes to download", "image", remoteRefStr, "byte", downloadBytes)
	return downloadBytes, nil
}

// missingLayers returns the layers present in the remote image but not already
// available locally, i.e. the layers that actually need to be downloaded.
func missingLayers(localRefStr string, remoteRefStr string) ([]dockerImageLayer, error) {
	localLayers, err := getImageLayers(localRefStr)
	if err != nil {
		return nil, err
	}

	remoteLayers, err := getImageLayers(remoteRefStr)
	if err != nil {
		return nil, err
	}

	localDigests := map[string]struct{}{}
	for _, l := range localLayers {
		localDigests[l.Hash] = struct{}{}
	}

	var missing []dockerImageLayer
	for _, l := range remoteLayers {
		if _, ok := localDigests[l.Hash]; ok {
			continue
		}
		missing = append(missing, l)
	}
	return missing, nil
}

// sumUniqueLayers sums the sizes of the layers across all images, counting each
// distinct layer (by digest) only once. Shared layers are downloaded a single
// time, so this yields the real total number of bytes to download.
func sumUniqueLayers(layersPerImage [][]dockerImageLayer) int64 {
	uniq := map[string]int64{}
	for _, layers := range layersPerImage {
		for _, l := range layers {
			uniq[l.Hash] = l.Size
		}
	}

	var total int64
	for _, size := range uniq {
		total += size
	}
	return total
}

type dockerImageLayer struct {
	Hash string
	Size int64
}

func getImageLayers(imageName string) ([]dockerImageLayer, error) {
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

	res := make([]dockerImageLayer, 0, len(imageLayers))
	for _, l := range imageLayers {
		hash, err := l.Digest()
		if err != nil {
			return nil, fmt.Errorf("error getting layer hash for %s: %w", imageName, err)
		}

		size, err := l.Size()
		if err != nil {
			return nil, fmt.Errorf("error getting size of layer %s: %w", hash.String(), err)
		}

		res = append(res, dockerImageLayer{Hash: hash.String(), Size: size})
	}

	return res, nil
}
