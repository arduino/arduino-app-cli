// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.bug.st/downloader/v3"
)

// downloadInactivityTimeout aborts a download when no data is received for this long.
const downloadInactivityTimeout = 30 * time.Second

// DownloadConfig returns the configuration to download the bricks index and the bricks.
func DownloadConfig(userAgent string) downloader.Config {
	return downloader.Config{
		ExtraHeaders:      map[string]string{"User-Agent": userAgent},
		InactivityTimeout: downloadInactivityTimeout,
	}
}

// downloadFile downloads url into the file dest, replacing it. When not zero, maxSize is the maximum
// number of bytes accepted and expectedSize is the size the server must declare, if it declares one.
// progress, if not nil, is called periodically with the number of bytes downloaded.
func downloadFile(ctx context.Context, config downloader.Config, url, dest string, expectedSize, maxSize int64, progress func(downloaded int64)) error {
	config.DoNotResumeDownload = true
	if maxSize != 0 {
		config.HttpClient.Transport = &maxBodyTransport{base: config.HttpClient.Transport, maxSize: maxSize}
	}
	config.AcceptFunc = func(head *http.Response) error {
		// The server may not support HEAD or may not declare the size: the body is checked anyway.
		if head.StatusCode != http.StatusOK || head.ContentLength < 0 {
			return nil
		}
		if expectedSize != 0 && head.ContentLength != expectedSize {
			return fmt.Errorf("size mismatch: expected %d bytes, the server declares %d", expectedSize, head.ContentLength)
		}
		return nil
	}
	if progress != nil {
		config.PollFunction = func(current, _ int64) { progress(current) }
	}
	if err := downloader.DownloadWithConfig(ctx, dest, url, config); err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	return nil
}

// maxBodyTransport makes reading a response body fail once it exceeds maxSize bytes.
type maxBodyTransport struct {
	base    http.RoundTripper
	maxSize int64
}

func (t *maxBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := cmp.Or(t.base, http.DefaultTransport).RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &maxBodyReader{ReadCloser: resp.Body, maxSize: t.maxSize}
	return resp, nil
}

type maxBodyReader struct {
	io.ReadCloser
	maxSize, read int64
}

func (r *maxBodyReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.read += int64(n)
	if r.read > r.maxSize {
		return 0, fmt.Errorf("the download is larger than %d bytes", r.maxSize)
	}
	return n, err
}
