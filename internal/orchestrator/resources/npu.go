// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
)

const npuBin = "qcnpuperf_cli"

// NPUMonitor runs qcnpuperf_cli in the background and exposes the latest
// max utilization via Percent().
type NPUMonitor struct {
	latest atomic.Uint32
}

// NewNPUMonitor starts qcnpuperf_cli and returns a monitor. Returns nil if
// the binary is not found.
func NewNPUMonitor(ctx context.Context) *NPUMonitor {
	cmd := exec.CommandContext(ctx, npuBin)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("npuperf-watch: stdout pipe failed", "error", err)
		return nil
	}
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil
		}
		slog.Error("npuperf-watch: failed to start", "error", err)
		return nil
	}

	m := &NPUMonitor{}
	go func() {
		defer func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				slog.Debug("qcnpuperf_cli exited", "error", err)
			}
		}()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			sample, err := parseNPULine(line)
			if err != nil {
				slog.Warn("npuperf-watch: failed to parse output", "line", line, "error", err)
				continue
			}
			m.latest.Store(math.Float32bits(max(sample.Q6Utilization, sample.HVXUtilization, sample.HMXUtilization)))
		}
	}()

	return m
}

// Percent returns the latest max NPU utilization percentage.
func (m *NPUMonitor) Percent() float32 {
	return math.Float32frombits(m.latest.Load())
}

// NPUSample holds all metrics from a single qcnpuperf_cli output line.
// Format: "NPU0 (CDSP) q6_utilization=39.73 q6_clock=768000 hvx_utilization=0.00 hmx_utilization=0.00"
type NPUSample struct {
	Q6Utilization  float32
	Q6Clock        uint64
	HVXUtilization float32
	HMXUtilization float32
}

func parseNPULine(line string) (NPUSample, error) {
	var s NPUSample
	found := 0

	for field := range strings.FieldsSeq(line) {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch key {
		case "q6_utilization":
			f, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return NPUSample{}, fmt.Errorf("invalid q6_utilization value %q: %w", val, err)
			}
			s.Q6Utilization = float32(f)
			found++
		case "q6_clock":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return NPUSample{}, fmt.Errorf("invalid q6_clock value %q: %w", val, err)
			}
			s.Q6Clock = n
			found++
		case "hvx_utilization":
			f, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return NPUSample{}, fmt.Errorf("invalid hvx_utilization value %q: %w", val, err)
			}
			s.HVXUtilization = float32(f)
			found++
		case "hmx_utilization":
			f, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return NPUSample{}, fmt.Errorf("invalid hmx_utilization value %q: %w", val, err)
			}
			s.HMXUtilization = float32(f)
			found++
		}
	}

	if found == 0 {
		return NPUSample{}, fmt.Errorf("no known fields found in line")
	}
	return s, nil
}
