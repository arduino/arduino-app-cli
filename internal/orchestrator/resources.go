// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

type SystemResource interface {
	systemResource() string // Private method makes this a sealed interface
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

type SystemMemoryResource struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

func (*SystemMemoryResource) systemResource() string { return "memory" }

type SystemResourceConfig struct {
	CPUScrapeInterval    time.Duration
	MemoryScrapeInterval time.Duration
	DiskScrapeInterval   time.Duration
}

func SystemResources(ctx context.Context, cfg config.Configuration, resourceCfg *SystemResourceConfig, cb func(SystemResource)) error {
	if resourceCfg == nil {
		resourceCfg = &SystemResourceConfig{
			CPUScrapeInterval:    time.Second * 5,
			MemoryScrapeInterval: time.Second * 5,
			DiskScrapeInterval:   time.Second * 30,
		}
	}

	firstMessagesToSend := []SystemResource{}
	memory, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	firstMessagesToSend = append(firstMessagesToSend, &SystemMemoryResource{Used: memory.Used, Total: memory.Total})

	cpuStats, err := cpu.Percent(0, false)
	if err != nil {
		return err
	}
	firstMessagesToSend = append(firstMessagesToSend, &SystemCPUResource{UsedPercent: cpuStats[0]})

	diskPaths := []string{"/", "/tmp", cfg.AppsDir().Parent().String()}
	for _, path := range diskPaths {
		diskStats, err := disk.Usage(path)
		if err != nil && !errors.Is(err, syscall.ENOENT) {
			return err
		}
		if diskStats != nil {
			firstMessagesToSend = append(firstMessagesToSend, &SystemDiskResource{Path: path, Used: diskStats.Used, Total: diskStats.Total})
		}
	}

	for _, msg := range firstMessagesToSend {
		cb(msg)
	}

	cpuTicker := time.NewTicker(resourceCfg.CPUScrapeInterval)
	defer cpuTicker.Stop()

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
			cb(&SystemCPUResource{UsedPercent: cpuStats[0]})
		case <-memoryTicker.C:
			memory, err := mem.VirtualMemory()
			if err != nil {
				slog.Warn("Failed to get memory usage", "error", err)
				continue
			}
			cb(&SystemMemoryResource{Used: memory.Used, Total: memory.Total})
		case <-diskTicker.C:
			for _, path := range diskPaths {
				diskStats, err := disk.Usage(path)
				if err != nil {
					slog.Warn("Failed to get disk usage", "path", path, "error", err)
					continue
				}
				cb(&SystemDiskResource{Path: path, Used: diskStats.Used, Total: diskStats.Total})
			}
		case <-ctx.Done():
			return nil
		}
	}
}
