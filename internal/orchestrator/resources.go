package orchestrator

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

func SystemResources(ctx context.Context, cfg *SystemResourceConfig) (iter.Seq[SystemResource], error) {
	if cfg == nil {
		cfg = &SystemResourceConfig{
			CPUScrapeInterval:    time.Second * 5,
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

	diskPaths := []string{"/", "/tmp", "/home/arduino"}
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

		cpuTicker := time.NewTicker(cfg.CPUScrapeInterval)
		defer cpuTicker.Stop()

		memoryTicker := time.NewTicker(cfg.MemoryScrapeInterval)
		defer memoryTicker.Stop()

		diskTicker := time.NewTicker(cfg.DiskScrapeInterval)
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
				return
			}
		}
	}, nil
}
