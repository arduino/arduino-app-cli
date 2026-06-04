package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// SysmonQueryProfData matches the C struct sysmon_query_prof_data exactly.
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
	DSP_NPU0                    int32 = 3
	RETURN_CODE_DSP_LIB_SUCCESS int32 = 0
)

var (
	qcomDspInit        func(int32) int32
	qcomDspGetProfData func(int32, *int32) unsafe.Pointer
	qcomDspDeinit      func(int32) int32
)

func init() {
	flag.Parse()
}

func main() {
	libPath := "/var/lib/arduino-app-cli/libqcnpuperf.so"
	duration := flag.Duration("duration", 10*time.Second, "how long to collect NPU stats")
	interval := flag.Duration("interval", 1*time.Second, "sampling interval")

	fmt.Printf("Opening %s\n", libPath)
	lib, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		log.Fatalf("Failed to open shared library: %v", err)
	}
	defer purego.Dlclose(lib)

	// Register the three NPU API functions
	purego.RegisterLibFunc(&qcomDspInit, lib, "qcom_dsp_init")
	purego.RegisterLibFunc(&qcomDspGetProfData, lib, "qcom_dsp_get_prof_data")
	purego.RegisterLibFunc(&qcomDspDeinit, lib, "qcom_dsp_deinit")

	// Initialize NPU
	fmt.Println("Calling qcom_dsp_init...")
	ret := qcomDspInit(DSP_NPU0)
	if ret != RETURN_CODE_DSP_LIB_SUCCESS {
		log.Fatalf("qcom_dsp_init failed with code %d", ret)
	}
	fmt.Println("✓ NPU initialized successfully")

	// Collect profiling data
	fmt.Printf("\nCollecting NPU stats for %v (interval: %v)...\n", *duration, *interval)
	fmt.Println("Time               Q6Util  HvxUtil HmxUtil")
	fmt.Println("---------- --------- ------- ------- -------")

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	deadline := time.Now().Add(*duration)
	sampleCount := 0

	for {
		now := time.Now()
		if now.After(deadline) {
			break
		}

		// Get profiling data
		var noMetrics int32
		ptr := qcomDspGetProfData(DSP_NPU0, &noMetrics)
		if ptr == nil {
			fmt.Printf("%s: ERROR - failed to get profiling data\n", now.Format("15:04:05.000"))
		} else if noMetrics <= 0 {
			fmt.Printf("%s: ERROR - no metrics returned (noMetrics=%d)\n", now.Format("15:04:05.000"), noMetrics)
		} else {
			data := (*SysmonQueryProfData)(ptr)
			fmt.Printf("%s %7.2f%% %7.2f%% %7.2f%%\n",
				now.Format("15:04:05.000"),
				data.Q6Utilization,
				data.HvxUtilization,
				data.HmxUtilization,
			)
			sampleCount++
		}

		select {
		case <-ticker.C:
		}
	}

	// Deinitialize NPU
	fmt.Printf("\nCollected %d samples. Calling qcom_dsp_deinit...\n", sampleCount)
	ret = qcomDspDeinit(DSP_NPU0)
	if ret != RETURN_CODE_DSP_LIB_SUCCESS {
		fmt.Fprintf(os.Stderr, "Warning: qcom_dsp_deinit returned code %d\n", ret)
	} else {
		fmt.Println("✓ NPU deinitialized successfully")
	}
}
