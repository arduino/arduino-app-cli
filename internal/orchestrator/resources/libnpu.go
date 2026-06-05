// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

// Go layout representing the C struct sysmon_query_prof_data.
// Must match C struct field order and size exactly.
type SysmonQueryProfData struct {
	Q6Utilization  float32
	Q6Clock        uint32
	Reserved0      float32
	HvxUtilization float32
	HmxUtilization float32
	Reserved1      float32
	Reserved2      float32
	Reserved3      float32
	Reserved4      float32
	Reserved5      float32
	Reserved6      float32
	Reserved7      float32
	Reserved8      float32
	Reserved9      float32
}

const (
	DSP_NPU0 int32 = 3 // CDSP_DOMAIN_ID
)

const (
	RETURN_CODE_DSP_LIB_SUCCESS int32 = 0
)

// There are the three enpoints provided by libqcnpuperf.so
var (
	qcomDspInit        func(int32) int32
	qcomDspGetProfData func(int32, *int32) unsafe.Pointer
	qcomDspDeinit      func(int32) int32
)

var libqcnpuperfLib uintptr
var libqcnpuperfLibraryPath = "/var/lib/arduino-app-cli/libqcnpuperf.so"

func initLibqcnpuperf() error {
	var err error
	libqcnpuperfLib, err = purego.Dlopen(libqcnpuperfLibraryPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("failed to open shared library: %v", err)
	}

	// Register function bindings with the correct C calling convention.
	purego.RegisterLibFunc(&qcomDspInit, libqcnpuperfLib, "qcom_dsp_init")
	purego.RegisterLibFunc(&qcomDspGetProfData, libqcnpuperfLib, "qcom_dsp_get_prof_data")
	purego.RegisterLibFunc(&qcomDspDeinit, libqcnpuperfLib, "qcom_dsp_deinit")

	return nil
}

func deInitLibqcnpuperfLib() {
	purego.Dlclose(libqcnpuperfLib)
	libqcnpuperfLib = uintptr(0)
}

func BoardSupportsNPU(platform platform.Platform) bool {
	return platform.BoardName == "ventunoq"
}
