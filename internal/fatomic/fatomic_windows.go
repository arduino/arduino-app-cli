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

package fatomic

import (
	"os"
)

// WriteFile this is used just to not break go build on Windows. We do not support
// atomic rename on Windows. In the scope of this project that aims to run only
// on Linux this function is only used to allow dev that runs on windows to test
// other part of the program.
func WriteFile(filename string, data []byte, perm os.FileMode, opts ...any) error {
	f, err := os.OpenFile(filename, os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	return nil
}
