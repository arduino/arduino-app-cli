package goffi

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
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

// Function pointers to be initialized
var (
	qcomDspInit        func(domain int32) int32
	qcomDspGetProfData func(domain int32, noMetrics *int32) unsafe.Pointer
	qcomDspDeinit      func(domain int32) int32
)

func initFFI() (error, func()) {
	// Load the shared library
	lib, err := ffi.LoadLibrary("/var/lib/arduino-app-cli/libqcnpuperf.so")
	if err != nil {
		return fmt.Errorf("failed to open shared library: %v", err), nil
	}

	cleanup := func() {
		defer ffi.FreeLibrary(lib)
	}

	err = initQcomDspInit(lib)
	if err != nil {
		return fmt.Errorf("failed to get library symbol: %v", err), cleanup
	}

	err = initQcomDspGetProfData(lib)
	if err != nil {
		return fmt.Errorf("failed to get library symbol: %v", err), cleanup
	}

	err = initQcomDspDeInit(lib)
	if err != nil {
		return fmt.Errorf("failed to get library symbol: %v", err), cleanup
	}

	return nil, cleanup
}

func initQcomDspInit(lib unsafe.Pointer) error {
	qcomDspInitSymbol, err := ffi.GetSymbol(lib, "qcom_dsp_init")
	if err != nil {
		return fmt.Errorf("failed to get library symbol: %v", err)
	}

	qcomDspInitCif := &types.CallInterface{}
	err = ffi.PrepareCallInterface(
		qcomDspInitCif,
		types.DefaultCall, // auto-detects platform calling convention (cdecl/stdcall/etc)
		types.SInt32TypeDescriptor,
		[]*types.TypeDescriptor{types.SInt32TypeDescriptor},
	)
	if err != nil {
		log.Fatalf("failed to make CIF for qcom_dsp_init: %v", err)
	}

	// init func
	qcomDspInit = func(domainId int32) int32 {
		var ret int32

		args := []unsafe.Pointer{
			unsafe.Pointer(&domainId),
		}

		err := ffi.CallFunction(qcomDspInitCif, qcomDspInitSymbol, unsafe.Pointer(&ret), args)
		if err != nil {
			log.Fatalf("FFI execution failed: %v", err)
		}
		return ret
	}

	return nil
}

func initQcomDspGetProfData(lib unsafe.Pointer) error {
	qcomGetProfDataSymbol, err := ffi.GetSymbol(lib, "qcom_dsp_get_prof_data")
	if err != nil {
		return fmt.Errorf("failed to get library symbol: %v", err)
	}

	qcomGetProfDataCif := &types.CallInterface{}
	err = ffi.PrepareCallInterface(
		qcomGetProfDataCif,
		types.DefaultCall,           // auto-detects platform calling convention
		types.PointerTypeDescriptor, // return type: pointer to struct
		[]*types.TypeDescriptor{types.SInt32TypeDescriptor, types.PointerTypeDescriptor}, // args: int32 domain, int32* no_metrics
	)
	if err != nil {
		log.Fatalf("failed to make CIF for qcom_dsp_get_prof_data: %v", err)
	}

	// init func
	qcomDspGetProfData = func(domain int32, noMetrics *int32) unsafe.Pointer {
		var ret unsafe.Pointer

		args := []unsafe.Pointer{
			unsafe.Pointer(&domain),    // Pass the value of domain by its pointer
			unsafe.Pointer(&noMetrics), // Pass the pointer to the pointer (*int32)
		}

		err := ffi.CallFunction(qcomGetProfDataCif, qcomGetProfDataSymbol, unsafe.Pointer(&ret), args)
		if err != nil {
			log.Fatalf("FFI execution failed: %v", err)
		}
		return ret
	}
	return nil
}

func initQcomDspDeInit(lib unsafe.Pointer) error {
	qcomDspDeInitSymbol, err := ffi.GetSymbol(lib, "qcom_dsp_deinit")
	if err != nil {
		return fmt.Errorf("failed to get library symbol: %v", err)
	}

	qcomDeInitCif := &types.CallInterface{}
	err = ffi.PrepareCallInterface(
		qcomDeInitCif,
		types.DefaultCall,          // auto-detects platform calling convention
		types.SInt32TypeDescriptor, // return type: int32
		[]*types.TypeDescriptor{types.SInt32TypeDescriptor}, // args: int32 domain
	)
	if err != nil {
		log.Fatalf("failed to make CIF for qcom_dsp_deinit: %v", err)
	}

	// init func
	qcomDspDeinit = func(domainId int32) int32 {
		var ret int32

		args := []unsafe.Pointer{
			unsafe.Pointer(&domainId),
		}

		err := ffi.CallFunction(qcomDeInitCif, qcomDspDeInitSymbol, unsafe.Pointer(&ret), args)
		if err != nil {
			log.Fatalf("FFI execution failed: %v", err)
		}
		return ret
	}

	return nil
}

func GetNpuPerformaceMain() error {
	// Lock this goroutine to a single OS thread for the entire DSP session.
	// This prevents RPC state issues if FastRPC is thread-sensitive.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err, cleanup := initFFI()
	if err != nil {
		return fmt.Errorf("failed to init PureGo: %v", err)
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	domain := DSP_NPU0
	fmt.Printf("Initializing DSP domain: %d\n", domain)

	ret := qcomDspInit(domain)
	if ret != RETURN_CODE_DSP_LIB_SUCCESS {
		return fmt.Errorf("qcom_dsp_init failed, ret=%d\n", ret)
	}

	fmt.Println("Getting profiling data...")
	var noMetrics int32
	for i := 0; i < 100000; i++ {
		ptr := qcomDspGetProfData(domain, &noMetrics)
		if ptr == nil || noMetrics <= 0 {
			fmt.Print("X\n")
			qcomDspDeinit(domain)
			return fmt.Errorf("Error getting profiling data")
		}

		// Cast to struct
		// data := (*SysmonQueryProfData)(ptr)
		// if i%1000 == 0 {
		// 	mem := getMemoryStats()
		// 	fmt.Printf("\nIter %d | NPU usage: %f | VmData: %d KB | VmSize: %d KB | VmRSS: %d KB\n",
		// 		i, data.Q6Utilization, mem.VmData, mem.VmSize, mem.VmRSS)
		// }

		if i%1000 == 0 {
			fmt.Print(".")
			os.Stdout.Sync() // Equivalent to fflush(stdout)
		}

		time.Sleep(1 * time.Microsecond)
	}

	fmt.Println("\nDeinitializing DSP...")
	ret = qcomDspDeinit(domain)
	if ret != RETURN_CODE_DSP_LIB_SUCCESS {
		return fmt.Errorf("Failed to deinit: error code %d\n", ret)
	}
	fmt.Println("DSP deinitialized successfully")
	return nil
}

type MemoryStats struct {
	VmData int64
	VmSize int64
	VmRSS  int64
}

func getMemoryStats() MemoryStats {
	var mem MemoryStats
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return mem
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "VmData:":
			mem.VmData, _ = strconv.ParseInt(fields[1], 10, 64)
		case "VmSize:":
			mem.VmSize, _ = strconv.ParseInt(fields[1], 10, 64)
		case "VmRSS:":
			mem.VmRSS, _ = strconv.ParseInt(fields[1], 10, 64)
		}
	}
	return mem
}
