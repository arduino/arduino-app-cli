// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"time"
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
