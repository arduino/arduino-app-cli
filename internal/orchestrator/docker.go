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

// Returns the highest version of a given docker image, from the input list, machiching the targetImage.
func GetHighestVersion(targetImage string, existingImages []string) string {
	targetBase := stripVersion(targetImage)

	var highestVer *semver.Version
	var highestImg = ""

	for _, img := range existingImages {
		if stripVersion(img) != targetBase {
			continue
		}

		v, err := semver.Parse(getTag(img))
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

// Returns a docker image name without the version.
func stripVersion(name string) string {
	if idx := strings.LastIndex(name, "@"); idx != -1 {
		return name[:idx]
	}
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		return name[:idx]
	}
	return name
}

// Returns the tag, or version, of a docker image.
func getTag(name string) string {
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		return name[idx+1:]
	}
	return ""
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
