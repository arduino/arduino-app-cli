package helpers

import (
	"fmt"
	"net"
	"strconv"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
)

func ArduinoCLIDownloadProgressToString(progress *rpc.DownloadProgress) string {
	switch {
	case progress.GetStart() != nil:
		return fmt.Sprintf("Download started: %s", progress.GetStart().GetUrl())
	case progress.GetUpdate() != nil:
		return fmt.Sprintf("Download progress: %s", progress.GetUpdate())
	case progress.GetEnd() != nil:
		return fmt.Sprintf("Download completed: %s", progress.GetEnd())
	}
	return progress.String()
}

func ArduinoCLITaskProgressToString(progress *rpc.TaskProgress) string {
	data := fmt.Sprintf("Task %s:", progress.GetName())
	if progress.GetMessage() != "" {
		data += fmt.Sprintf(" (%s)", progress.GetMessage())
	}
	if progress.GetCompleted() {
		data += " completed"
	} else {
		data += fmt.Sprintf(" %.2f%%", progress.GetPercent())
	}
	return data
}

func GetHostIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	getIP := func(name string) (string, error) {
		for _, iface := range ifaces {
			if iface.Name == name {
				addrs, err := iface.Addrs()
				if err != nil {
					return "", err
				}
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						return ipnet.IP.String(), nil
					}
				}
			}
		}
		return "", fmt.Errorf("no IP address found for %s", name)
	}

	if ip, err := getIP("eth0"); err == nil {
		return ip, nil
	}

	return getIP("wlan0")
}

func ToHumanMiB(bytes int64) string {
	return strconv.FormatFloat(float64(bytes)/(1024.0*1024.0), 'f', 2, 64) + "MiB"
}
