// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

type SystemResource interface {
	systemResource() string
}

type SystemDiskResource struct {
	Path  string `json:"path"`
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

func (*SystemDiskResource) systemResource() string { return "disk" }

type SystemCPUResource struct {
	UsedPercent float64 `json:"used_percent"`
}

func (*SystemCPUResource) systemResource() string { return "cpu" }

type SystemNPUResource struct {
	UsedPercent float32 `json:"max_percent"`
}

func (*SystemNPUResource) systemResource() string { return "npu" }

type SystemMemoryResource struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

func (*SystemMemoryResource) systemResource() string { return "memory" }

type SystemResourceConfig struct {
	CPUScrapeInterval    time.Duration
	NPUScrapeInterval    time.Duration
	MemoryScrapeInterval time.Duration
	DiskScrapeInterval   time.Duration
}

func SystemResources(ctx context.Context, cfg config.Configuration, resourceCfg *SystemResourceConfig, hasNPU bool) (iter.Seq[SystemResource], error) {
	if resourceCfg == nil {
		resourceCfg = &SystemResourceConfig{
			CPUScrapeInterval:    time.Second * 5,
			NPUScrapeInterval:    time.Second * 5,
			MemoryScrapeInterval: time.Second * 5,
			DiskScrapeInterval:   time.Second * 30,
		}
	}

	firstMessagesToSend := []SystemResource{}
	memory, err := mem.VirtualMemory()
	if err != nil {
		return helpers.EmptyIter[SystemResource](), err
	}
	firstMessagesToSend = append(firstMessagesToSend, &SystemMemoryResource{Used: memory.Used, Total: memory.Total})

	cpuStats, err := cpu.Percent(0, false)
	if err != nil {
		return helpers.EmptyIter[SystemResource](), err
	}
	firstMessagesToSend = append(firstMessagesToSend, &SystemCPUResource{UsedPercent: cpuStats[0]})

	if hasNPU {
		if err := NPUInit(); err != nil {
			slog.Error("Failed to init NPU", "error", err)
		}
		npuStats, err := NPUPercent()
		if err != nil {
			slog.Error("Failed to get NPU metrics", "error", err)
		}
		firstMessagesToSend = append(firstMessagesToSend, &SystemNPUResource{UsedPercent: npuStats})
	}

	diskPaths := []string{"/", "/tmp", cfg.AppsDir().Parent().String()}
	for _, path := range diskPaths {
		diskStats, err := disk.Usage(path)
		if err != nil && !errors.Is(err, syscall.ENOENT) {
			return helpers.EmptyIter[SystemResource](), err
		}
		if diskStats != nil {
			firstMessagesToSend = append(firstMessagesToSend, &SystemDiskResource{Path: path, Used: diskStats.Used, Total: diskStats.Total})
		}
	}

	return func(yield func(SystemResource) bool) {
		for _, msg := range firstMessagesToSend {
			if !yield(msg) {
				return
			}
		}

		cpuTicker := time.NewTicker(resourceCfg.CPUScrapeInterval)
		defer cpuTicker.Stop()

		var npuTicker *time.Ticker
		var npuChannel <-chan time.Time
		if hasNPU {
			npuTicker = time.NewTicker(resourceCfg.NPUScrapeInterval)
			defer npuTicker.Stop()
			npuChannel = npuTicker.C
		}

		memoryTicker := time.NewTicker(resourceCfg.MemoryScrapeInterval)
		defer memoryTicker.Stop()

		diskTicker := time.NewTicker(resourceCfg.DiskScrapeInterval)
		defer diskTicker.Stop()

		for {
			select {
			case <-cpuTicker.C:
				cpuStats, err := cpu.Percent(0, false)
				if err != nil {
					slog.Warn("Failed to get CPU usage", "error", err)
					continue
				}
				if !yield(&SystemCPUResource{UsedPercent: cpuStats[0]}) {
					return
				}
			case <-npuChannel:
				npuStats, err := NPUPercent()
				if err != nil {
					slog.Warn("Failed to get NPU usage", "error", err)
					continue
				}
				if !yield(&SystemNPUResource{UsedPercent: npuStats}) {
					return
				}
			case <-memoryTicker.C:
				memory, err := mem.VirtualMemory()
				if err != nil {
					slog.Warn("Failed to get memory usage", "error", err)
					continue
				}
				if !yield(&SystemMemoryResource{Used: memory.Used, Total: memory.Total}) {
					return
				}
			case <-diskTicker.C:
				for _, path := range diskPaths {
					diskStats, err := disk.Usage(path)
					if err != nil {
						slog.Warn("Failed to get disk usage", "path", path, "error", err)
						continue
					}
					if !yield(&SystemDiskResource{Path: path, Used: diskStats.Used, Total: diskStats.Total}) {
						return
					}
				}
			case <-ctx.Done():
				if hasNPU {
					err := NPUDeInit()
					if err != nil {
						slog.Warn("Failed to deinit NPU DSP", "error", err)

					}

				}
				return
			}
		}
	}, nil
}
