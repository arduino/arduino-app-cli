// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

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
