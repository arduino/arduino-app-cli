// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	dockerClient "github.com/moby/moby/client"
	"github.com/shirou/gopsutil/v4/disk"

	"github.com/arduino/arduino-app-cli/internal/helpers"
)

// ErrOutOfSpace is what a download the board has no room for answers.
var ErrOutOfSpace = errors.New("not enough disk space to pull the docker image")

// images required by the system

// PullImages downloads the images the board has not, as one download: the size comes
// from the registry, so the percentage is over what is really left to fetch.
func PullImages(ctx context.Context, docker dockerClient.APIClient, images []string, line func(string), progress func(label string, curr, total int64)) error {
	if line == nil {
		line = func(string) {}
	}
	if progress == nil {
		progress = func(string, int64, int64) {}
	}
	pulledImages, err := ListImages(ctx, docker)
	if err != nil {
		return err
	}
	imagesToPull := slices.DeleteFunc(slices.Clone(images), func(image string) bool {
		return slices.Contains(pulledImages, image)
	})
	if len(imagesToPull) == 0 {
		return nil
	}

	allLayers := make([]dockerImageLayer, 0, len(imagesToPull))
	for _, image := range imagesToPull {
		layers, err := missingLayers(highestVersion(image, pulledImages), image)
		if err != nil {
			slog.Warn("Unable to get the new image layers size", "image", image, "error", err)
		}
		allLayers = append(allLayers, layers...)
	}
	// Shared layers are counted once, so this is the real download size.
	totalBytes := sumUniqueLayers(allLayers)
	slog.Info("total docker images download size", "bytes", totalBytes)

	freeSpace, err := dockerFreeSpace()
	if err != nil {
		return err
	}
	if uint64(float64(totalBytes)*2.5) > freeSpace {
		return ErrOutOfSpace
	}

	layerProgress := map[string]int64{}
	var reported helpers.LastPercent
	var lastLabel string
	for i, image := range imagesToPull {
		line(fmt.Sprintf("Pulling container image %s ...", image))
		// The percentage stays global (across all images); the label tells the
		// user which image is currently being pulled.
		lastLabel = fmt.Sprintf("Pulling image %d/%d (%s)", i+1, len(imagesToPull), imageName(image))
		if err := pullImage(ctx, docker, image, layerProgress, totalBytes, &reported, func(downloaded int64) {
			progress(lastLabel, downloaded, totalBytes)
		}); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}
	}

	if totalBytes > 0 {
		progress(lastLabel, totalBytes, totalBytes)
	}
	return nil
}

// ListImages reports which images of ImagePrefixes the board already has.
func ListImages(ctx context.Context, docker dockerClient.APIClient) ([]string, error) {
	images, err := docker.ImageList(ctx, dockerClient.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(images.Items))
	for _, image := range images.Items {
		for _, tag := range image.RepoTags {
			if slices.ContainsFunc(ImagePrefixes, func(p string) bool {
				return strings.HasPrefix(tag, p)
			}) {
				result = append(result, tag)
			}
		}
	}

	return result, nil
}

func RemoveImage(ctx context.Context, docker dockerClient.APIClient, imageName string) (int64, error) {
	var size int64
	if info, err := docker.ImageInspect(ctx, imageName); err != nil {
		slog.Warn("failed to inspect image", "image", imageName, "error", err)
	} else {
		size = info.Size
	}

	if _, err := docker.ImageRemove(ctx, imageName, dockerClient.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	}); err != nil {
		return 0, fmt.Errorf("failed to remove image %s: %w", imageName, err)
	}

	return size, nil
}

// ImagePrefixes states which images are ours, past ones included: what to pull, and
// what a cleanup may remove.
var ImagePrefixes = []string{
	"ghcr.io/bcmi-labs/",
	"public.ecr.aws/arduino/",
	"ghcr.io/arduino/",
	"influxdb",
	"artifacts.codelinaro.org/iot-solutions-microservices/",
}

// dockerFreeSpace is the free space of the partition where docker keeps its images.
func dockerFreeSpace() (uint64, error) {
	usage, err := disk.Usage("/var/lib/docker")
	if err != nil {
		return 0, err
	}

	return usage.Free, nil
}

// updateLayerProgress records the bytes downloaded so far for a single layer
func updateLayerProgress(layerProgress map[string]int64, status, id string, current int64) int64 {
	if status == "Downloading" && id != "" {
		layerProgress[id] = current
	}

	var downloaded int64
	for _, c := range layerProgress {
		downloaded += c
	}
	return downloaded
}

const minDelay = 1 * time.Second
const maxDelay = 10 * time.Second

func pullImage(ctx context.Context, docker dockerClient.APIClient, imageName string, layerProgress map[string]int64, totalBytes int64, reported *helpers.LastPercent, progress func(downloaded int64)) error {
	delay := minDelay
	var out io.ReadCloser
	var allErr error
	var lastErr error
	for range 10 { // 1s, 2s, 4s, 8s, 10s, 10s, 10s, 10s, 10s, 10s
		out, lastErr = docker.ImagePull(ctx, imageName, dockerClient.ImagePullOptions{})
		if lastErr == nil {
			break // Success
		}
		allErr = errors.Join(allErr, lastErr)

		if !isTemporaryDockerError(lastErr) {
			return allErr // Non-retryable error
		}

		slog.Warn("received 'toomanyrequests' error from Docker registry, retrying", "delay", delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay = min(delay*2, maxDelay)
	}
	if lastErr != nil {
		return fmt.Errorf("failed to pull image %s after multiple attempts: %w", imageName, allErr)
	}
	defer out.Close()

	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		type Payload struct {
			Status         string `json:"status"`
			Progress       string `json:"progress"`
			ID             string `json:"id"`
			ProgressDetail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
		}

		var payload Payload
		if err := json.Unmarshal(scanner.Bytes(), &payload); err == nil {
			// Accumulate the downloaded bytes across all layers/images and report
			// the global download progress.
			downloaded := min(updateLayerProgress(layerProgress, payload.Status, payload.ID, payload.ProgressDetail.Current), totalBytes)
			if reported.Moved(downloaded, totalBytes) {
				progress(downloaded)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func isTemporaryDockerError(err error) bool {
	errorString := err.Error()
	transientSubstrings := []string{
		"toomanyrequests",
		"Client.Timeout exceeded",
		"request canceled while waiting for connection",
	}

	for _, sub := range transientSubstrings {
		if strings.Contains(errorString, sub) {
			return true
		}
	}
	return false
}
