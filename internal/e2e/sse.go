// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package e2e

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"
)

// Event is a Server-Sent Event as delivered by the daemon.
type Event struct {
	ID    string
	Event string
	Data  []byte // json
}

// ParseSSE parses a Server-Sent Events stream from body, yielding one Event
// per frame. It terminates when the body returns an error (including io.EOF).
func ParseSSE(body io.Reader) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		reader := bufio.NewReader(body)
		evt := Event{}
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				_ = yield(Event{}, err)
				return
			}
			switch {
			case strings.HasPrefix(line, "data:"):
				evt.Data = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			case strings.HasPrefix(line, "event:"):
				evt.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "id:"):
				evt.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			case strings.HasPrefix(line, "\n"):
				if !yield(evt, nil) {
					return
				}
				evt = Event{}
			default:
				_ = yield(Event{}, fmt.Errorf("unknown line: '%s'", line))
				return
			}
		}
	}
}

// NewSSEClient opens a GET request against url and parses the response as a
// Server-Sent Events stream.
func NewSSEClient(ctx context.Context, url string) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			_ = yield(Event{}, err)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			_ = yield(Event{}, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			_ = yield(Event{}, fmt.Errorf("got response status code %d", resp.StatusCode))
			return
		}

		for event, err := range ParseSSE(resp.Body) {
			if !yield(event, err) {
				return
			}
		}
	}
}
