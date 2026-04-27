package board

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

type KeyboardLayout struct {
	LayoutId    string
	Description string
}

func GetKeyboardLayout(ctx context.Context, conn remote.RemoteConn) (string, error) {
	cmd := conn.GetCmd("localectl", "status")
	output, err := cmd.Output(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get the keyboard layout: %w", err)
	}

	lines := strings.Split(string(output), "\n")

	// Loop through each line of the output to find "X11 Layout"
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "X11 Layout:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return "", fmt.Errorf("failed to get the keyboard layout, parsing error")
			}

			layout := strings.TrimSpace(parts[1])
			return layout, nil
		}
	}

	return "", fmt.Errorf("failed to get the keyboard layout, layout not found")
}

func SetKeyboardLayout(ctx context.Context, conn remote.RemoteConn, layoutCode string) error {
	err := conn.GetCmd("sudo", "/usr/local/bin/arduino-set-keyboard-layout", layoutCode).Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	return nil
}

func ListKeyboardLayouts(conn remote.RemoteConn) ([]KeyboardLayout, error) {
	// The file contains multiple things, including the list of valid keyboard layouts.
	r, err := conn.ReadFile("/usr/share/X11/xkb/rules/base.lst")
	if err != nil {
		return nil, fmt.Errorf("failed opening the keyboard layouts file: %w", err)
	}
	defer r.Close()

	var layouts []KeyboardLayout

	scanner := bufio.NewScanner(r)
	insideLayoutSection := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "! layout") {
			insideLayoutSection = true
			continue
		}

		if !insideLayoutSection {
			continue
		}

		// If the line is empty or starts with "!", it's the end of the layout section
		if line == "" || strings.HasPrefix(line, "!") {
			break
		}

		// Split the line into layout code and description
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			layout := KeyboardLayout{
				LayoutId:    parts[0],
				Description: strings.Join(parts[1:], " "),
			}
			layouts = append(layouts, layout)
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading input: %v", err)
	}

	return layouts, nil
}
