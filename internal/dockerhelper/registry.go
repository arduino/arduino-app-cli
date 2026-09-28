// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// What an image is made of, read from the registry and not from the engine: it is how
// the size of a download is known before it starts.

package dockerhelper

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	semver "go.bug.st/relaxed-semver"
)

// parseDockerImage splits an image in its name and its tag or version.
func parseDockerImage(image string) (name string, version string) {
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		return image[:idx], image[idx+1:]
	}
	if idx := strings.LastIndex(image, ":"); idx != -1 {
		return image[:idx], image[idx+1:]
	}
	return image, ""
}

// imageName is the reference without its tag or digest.
func imageName(image string) string {
	name, _ := parseDockerImage(image)
	return name
}

// missingLayers is what the remote image has and the local one has not: the layers a
// pull has to download.
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

// sumUniqueLayers counts every digest once: a layer shared by two images is downloaded
// once, so this is the real size of a download.
func sumUniqueLayers(layers []dockerImageLayer) int64 {
	uniq := map[string]int64{}
	for _, l := range layers {
		uniq[l.Hash] = l.Size
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

// highestVersion is the newest version of the image in the list, or an empty string.
func highestVersion(targetImage string, existingImages []string) string {
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
