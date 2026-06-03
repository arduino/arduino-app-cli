package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
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
	qcomDspInit        func(int32) int32
	qcomDspGetProfData func(int32, *int32) unsafe.Pointer
	qcomDspDeinit      func(int32) int32
)

func initPureGoCalls() (error, func()) {
	// Load the shared library
	lib, err := purego.Dlopen("/var/lib/arduino-app-cli/libqcnpuperf.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("failed to open shared library: %v", err), nil
	}

	cleanup := func() {
		purego.Dlclose(lib)
	}

	// Register function bindings with the correct C calling convention.
	purego.RegisterLibFunc(&qcomDspInit, lib, "qcom_dsp_init")
	purego.RegisterLibFunc(&qcomDspGetProfData, lib, "qcom_dsp_get_prof_data")
	purego.RegisterLibFunc(&qcomDspDeinit, lib, "qcom_dsp_deinit")

	return nil, cleanup
}

// TODO/open points to be discussed
//   - add sudo groupadd fastrpc because: chmod 666 /dev/fastrpc-cdsp-secure /dev/fastrpc-cdsp
//     better using groups
//   - move init servicelocator.go (will this called by daemon and cli mode?)
//   - where to call deinit at shutdown
//   - if the deinit is not called is there someone in charge to deallocate the DSP allocated memory?

func main() {
	// 1. Define a hidden CLI flag specifically for the sub-processes
	workerFlag := flag.Int("run-worker-stream", -1, "Internal use only: runs a stream worker")
	flag.Parse()

	// 2. CHECK: If the flag is set, we are a worker process!
	// Execute the stream function and exit immediately.
	if *workerFlag != -1 {
		stream(*workerFlag)
		return
	}

	// 3. MAIN PROCESS LOGIC: If the flag is NOT set, we are the orchestrator.
	// We will spawn N copies of ourselves.
	n := 3
	var wg sync.WaitGroup

	// Find the path to our own currently running executable binary
	selfPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to find self executable path: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[Orchestrator PID %d] Spawning %d parallel OS processes...\n", os.Getpid(), n)

	for i := 1; i <= n; i++ {
		wg.Add(1)
		streamID := i

		go func() {
			defer wg.Done()

			// Create a command that executes OUR OWN binary, but passes the secret worker flag
			cmd := exec.Command(selfPath, fmt.Sprintf("-run-worker-stream=%d", streamID))

			// Pipe the sub-process output directly to your current terminal
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Run and wait for this specific OS process to exit
			if err := cmd.Run(); err != nil {
				fmt.Printf("Worker process for Stream %d failed: %v\n", streamID, err)
			}
		}()
	}

	wg.Wait()
	fmt.Println("All parallel OS processes have exited cleanly.")
}

func stream(streamID int) error {
	// Lock this goroutine to a single OS thread for the entire DSP session.
	// This prevents RPC state issues if FastRPC is thread-sensitive.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err, cleanup := initPureGoCalls()
	if err != nil {
		return fmt.Errorf("streamID %d failed to init PureGo: %v", streamID, err)
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	domain := DSP_NPU0
	fmt.Printf("streamID %d Initializing DSP domain: %d\n", streamID, domain)

	ret := qcomDspInit(domain)
	if ret != RETURN_CODE_DSP_LIB_SUCCESS {
		return fmt.Errorf("streamID %d qcom_dsp_init failed, ret=%d\n", streamID, ret)
	}

	fmt.Println("Getting profiling data...")
	var noMetrics int32

	for i := 0; i < 100000; i++ {
		ptr := qcomDspGetProfData(domain, &noMetrics)
		if ptr == nil || noMetrics <= 0 {
			fmt.Printf("streamID %d X\n", streamID)
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
			fmt.Print(streamID)
			os.Stdout.Sync() // Equivalent to fflush(stdout)
		}

		time.Sleep(1 * time.Microsecond)
	}

	fmt.Printf("streamID %d Deinitializing DSP...\n", streamID)
	ret = qcomDspDeinit(domain)
	if ret != RETURN_CODE_DSP_LIB_SUCCESS {
		return fmt.Errorf("Failed to deinit: error code %d\n", ret)

	}

	fmt.Printf("\n streamID %d DSP deinitialized successfully\n", streamID)
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
