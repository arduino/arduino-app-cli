// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"fmt"
	"runtime"
	"sync"
)

/*
The fastrpc kernel library stores initialization state tied to a specific OS thread ID.
Go's runtime can reschedule a goroutine to different OS threads between function calls,
breaking fastrpc's thread-local state assumptions.

Create a dedicated worker goroutine, locked via runtime.LockOSThread(),
to ensures all NPU operations execute on the same thread, preventing Go
from migrating the goroutine.

There is one worker with three operations types, one for each libqcnpuperf API call, which run
mutually exclusively to prevent stream collisions.

A reference counter protects the resources, preventing deallocation while
there are still active streams.
*/

type npuRequestType int

const (
	npuReqInit npuRequestType = iota
	npuReqMaxUsage
)

type npuRequest struct {
	responseType npuRequestType
	err          chan error
	maxUsage     chan float32
}

var (
	npuWorkerOnce sync.Once
	npuRequests   chan npuRequest
)

func startNPUWorker() {
	npuWorkerOnce.Do(func() {
		npuRequests = make(chan npuRequest)
		go npuWorker()
	})
}

// npuWorker is the dedicated goroutine that locks to a single OS thread.
// It processes all NPU requests sequentially, ensuring thread-local state consistency.
func npuWorker() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for req := range npuRequests {
		switch req.responseType {
		case npuReqInit:
			req.err <- npuInitImpl()
		case npuReqMaxUsage:
			val, err := npuPercentImpl()
			if err != nil {
				req.err <- err
			} else {
				req.maxUsage <- val
				req.err <- nil
			}
		}
	}
}

// Initializes the npu DSP domain only once.
// Increments reference counter to keep trace of active streams.
func NPUInit() error {
	startNPUWorker()
	req := npuRequest{
		responseType: npuReqInit,
		err:          make(chan error, 1),
	}
	npuRequests <- req
	return <-req.err
}

func npuInitImpl() error {
	if err := initLibqcnpuperf(); err != nil {
		return err
	}
	ret := qcomDspInit(DSP_NPU0)
	if ret != RETURN_CODE_DSP_LIB_SUCCESS {
		return fmt.Errorf("qcom_dsp_init failed, ret=%d", ret)
	}
	return nil
}

// Returns the current NPU utilization percentage
func NPUPercent() (float32, error) {
	req := npuRequest{
		responseType: npuReqMaxUsage,
		err:          make(chan error, 1),
		maxUsage:     make(chan float32, 1),
	}
	npuRequests <- req
	if err := <-req.err; err != nil {
		return 0, err
	}
	return <-req.maxUsage, nil
}

func npuPercentImpl() (float32, error) {
	var noMetrics int32
	ptr := qcomDspGetProfData(DSP_NPU0, &noMetrics)
	if ptr == nil || noMetrics <= 0 {
		return 0, fmt.Errorf("qcomDspGetProfData error: no profile data available")
	}

	data := (*SysmonQueryProfData)(ptr)
	return max(data.Q6Utilization, data.HmxUtilization, data.HvxUtilization), nil
}
