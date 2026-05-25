package orchestrator

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

// SysmonQueryProfData matches the C struct layout
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
	DspNpu0 = 3 // CDSP_DOMAIN_ID
)

// DspReturnCode enum
const (
	ReturnCodeSuccess           = 0
	ReturnCodeLibFail           = 1
	ReturnCodeOpenFailed        = 2
	ReturnCodeInitFailed        = 3
	ReturnCodeRpcMemAllocFailed = 4
	ReturnCodeGetProfDataFailed = 5
	ReturnCodeDeinitFailed      = 6
)

// TODO/open points to be discussed
//   - add sudo groupadd fastrpc because: chmod 666 /dev/fastrpc-cdsp-secure /dev/fastrpc-cdsp
//     better using groups
//   - create the package with the library
//   - refactor, move init/deinit at startup/shutdown
//   - if the deinit is not called is there someone in charge to deallocate the DSP allocated memory?
//   - fix return error
//   - we call this once, as daemon, and several times as client, we use npu metrics from daemon mode,
//     we should call init from daemon mode only
//   - We can have more than one board, served from the same arduino-app-cli daemon instance. How to handle
//     per board memory allocation?
func GetDataAll() string {
	// Load the shared library
	lib, err := purego.Dlopen("/var/lib/arduino-app-cli/libqcnpuperf.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		fmt.Printf("Failed to load library: %v\n", err)
		return "ERROR"
	}
	defer purego.Dlclose(lib)

	// Define function signatures
	var qcomDspInit func(int32) int32
	var qcomDspGetProfData func(int32, *int32) unsafe.Pointer
	var qcomDspDeinit func(int32) int32

	// Register function bindings with the correct C calling convention.
	purego.RegisterLibFunc(&qcomDspInit, lib, "qcom_dsp_init")
	purego.RegisterLibFunc(&qcomDspGetProfData, lib, "qcom_dsp_get_prof_data")
	purego.RegisterLibFunc(&qcomDspDeinit, lib, "qcom_dsp_deinit")

	// Initialize
	fmt.Println("Initializing DSP...")
	initResult := qcomDspInit(DspNpu0)
	if initResult != ReturnCodeSuccess {
		fmt.Printf("Failed to initialize: error code %d\n", initResult)
		return "Failed to initialize: error code"
	}

	// Get profiling data
	fmt.Println("Getting profiling data...")
	var noMetrics int32
	profDataPtr := qcomDspGetProfData(DspNpu0, &noMetrics)
	if profDataPtr == nil {
		fmt.Println("Failed to get profiling data")
		qcomDspDeinit(DspNpu0)
		return "Failed to get profiling data"
	}

	// Cast to struct
	profData := (*SysmonQueryProfData)(profDataPtr)

	// Print data
	res := getOutput(profData, int(noMetrics))

	// Cleanup
	fmt.Println("\nDeinitializing DSP...")
	deinitResult := qcomDspDeinit(DspNpu0)
	if deinitResult != ReturnCodeSuccess {
		fmt.Printf("Failed to deinit: error code %d\n", deinitResult)
		return "Failed to deinit: error code"
	}

	fmt.Println("Done!")
	return res
}

func getOutput(profData *SysmonQueryProfData, noMetrics int) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "\n=== Profiling Data ===\n")
	fmt.Fprintf(&builder, "Q6 Utilization:   %.2f %%\n", profData.Q6Utilization)
	fmt.Fprintf(&builder, "Q6 Clock:         %d KHz\n", profData.Q6Clock)
	fmt.Fprintf(&builder, "HVX Utilization:  %.2f %%\n", profData.HvxUtilization)
	fmt.Fprintf(&builder, "HMX Utilization:  %.2f %%\n", profData.HmxUtilization)
	fmt.Fprintf(&builder, "Number of Metrics: %d\n", noMetrics)

	return builder.String()
}
