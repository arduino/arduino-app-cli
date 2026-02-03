package update

import (
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// New plan:
// Modify the system init function, so that it will check the free space in the docker partition
// then for each image to pull, estimate the amount of bytes needed for the pull to complete
// this is done comparing new with old (if old is present) otherwise just get the layers of new
// if free space is enough, proceed with the pull
// otherwise return an error with a specific status code (the caller must be updated to recognise the code)
// the caller (upgrade code) runs system cleanup and then tries system init again
// the system init repeats this for all the images to pull

// Returns the number of bytes that would be downloaded when pulling the new docker image while the old one is
// already present locally. It accounts for image layers that are already present locally.
func getBytesToDownload(localRefStr string, remoteRefStr string) (int64, error) {
	localRef, err := name.ParseReference(localRefStr)
	if err != nil {
		log.Fatal(err)
	}

	remoteRef, err := name.ParseReference(remoteRefStr)
	if err != nil {
		log.Fatal(err)
	}

	// Fetch manifests
	localImg, err := remote.Image(localRef)
	if err != nil {
		log.Fatalf("fetch local manifest: %v", err)
	}

	remoteImg, err := remote.Image(remoteRef)
	if err != nil {
		log.Fatalf("fetch remote manifest: %v", err)
	}

	// Collect local layer digests
	localLayers, err := localImg.Layers()
	if err != nil {
		log.Fatal(err)
	}

	localDigests := map[string]struct{}{}

	for _, l := range localLayers {
		h, err := l.Digest()
		if err != nil {
			log.Fatal(err)
		}
		localDigests[h.String()] = struct{}{}
	}

	// Compare with remote
	var downloadBytes int64

	remoteLayers, err := remoteImg.Layers()
	if err != nil {
		log.Fatal(err)
	}

	for i, l := range remoteLayers {
		h, err := l.Digest()
		if err != nil {
			log.Fatal(err)
		}

		size, err := l.Size()
		if err != nil {
			log.Fatal(err)
		}

		if _, ok := localDigests[h.String()]; ok {
			fmt.Printf(
				"[%02d] PRESENT  %s (%d bytes)\n",
				i, h, size,
			)
			continue
		}

		fmt.Printf(
			"[%02d] MISSING  %s (%d bytes)\n",
			i, h, size,
		)
		downloadBytes += size
	}

	return downloadBytes / 1024, nil
}
