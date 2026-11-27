// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/sirupsen/logrus"

	"github.com/arduino/arduino-app-cli/internal/helpers"
)

// arduinoCLI is the arduino-cli of this process: one server and one instance, kept
// initialized for the last sketch profile served, so that a build of the same sketch does
// not load the platform and resolve the libraries again. A board query or an upload takes
// the instance as it is, with or without a profile.
type arduinoCLI struct {
	mu       sync.Mutex
	srv      rpc.ArduinoCoreServiceServer
	inst     *rpc.Instance
	dataDir  *paths.Path
	profile  string // the sketch key the instance was initialized for, noProfile without one
	hardware string // the installed platforms the instance loaded
}

const noProfile = "-"

var cli = &arduinoCLI{}

// forSketch runs fn on the instance initialized for the profile of the sketch, an init that
// is skipped when the instance already serves that profile. w gets the output of the init.
func (c *arduinoCLI) forSketch(ctx context.Context, sketchPath *paths.Path, w io.Writer, fn instanceFunc) error {
	return c.run(ctx, sketchPath, w, fn)
}

// forPlatform runs fn on the instance as it is, with or without a profile: the installed
// platform answers a board query or an upload the same either way.
func (c *arduinoCLI) forPlatform(ctx context.Context, fn instanceFunc) error {
	return c.run(ctx, nil, io.Discard, fn)
}

type instanceFunc func(srv rpc.ArduinoCoreServiceServer, inst *rpc.Instance) error

// run holds the instance for the whole of fn: one caller at a time, and fn must not come back
// for the instance.
func (c *arduinoCLI) run(ctx context.Context, sketchPath *paths.Path, w io.Writer, fn instanceFunc) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.initialize(ctx, sketchPath, w); err != nil {
		return err
	}
	return fn(c.srv, c.inst)
}

func (c *arduinoCLI) initialize(ctx context.Context, sketchPath *paths.Path, w io.Writer) error {
	if c.srv == nil {
		if err := c.create(ctx); err != nil {
			return err
		}
	}

	// The instance is initialized again when the sketch profile to serve changes and when
	// the installed platforms did.
	hardware := c.installedHardware()
	profile := noProfile
	initReq := &rpc.InitRequest{Instance: c.inst}
	if sketchPath != nil {
		var err error
		if profile, err = sketchKey(sketchPath); err != nil {
			return err
		}
		sketchResp, err := c.srv.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath.String()})
		if err != nil {
			return err
		}
		initReq.Profile = sketchResp.GetSketch().GetDefaultProfile().GetName()
		if initReq.Profile == "" {
			return fmt.Errorf("sketch %q has no default profile", sketchPath)
		}
		initReq.SketchPath = sketchPath.String()
	} else if c.profile != "" && c.hardware == hardware {
		return nil
	}
	if c.profile == profile && c.hardware == hardware {
		return nil
	}

	c.profile, c.hardware = "", ""
	if err := c.srv.Init(initReq, commands.InitStreamResponseToCallbackFunction(ctx, initProgressWriter(w))); err != nil {
		return err
	}
	c.profile, c.hardware = profile, hardware
	return nil
}

func (c *arduinoCLI) create(ctx context.Context) error {
	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return err
	}
	setting, err := srv.SettingsGetValue(ctx, &rpc.SettingsGetValueRequest{Key: "directories.data", ValueFormat: "json"})
	if err != nil {
		return err
	}
	var dataDir string
	if err := json.Unmarshal([]byte(setting.GetEncodedValue()), &dataDir); err != nil {
		return fmt.Errorf("cannot read the arduino-cli data folder: %w", err)
	}
	resp, err := srv.Create(ctx, &rpc.CreateRequest{})
	if err != nil {
		return err
	}
	c.srv, c.inst, c.dataDir = srv, resp.GetInstance(), paths.New(dataDir)
	return nil
}

// platforms names the installed platforms with their versions, an input of the sketch
// fingerprint. Not for use inside forSketch or forPlatform.
func (c *arduinoCLI) platforms(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.srv == nil {
		if err := c.create(ctx); err != nil {
			return "", err
		}
	}
	return c.installedHardware(), nil
}

// installedHardware names the platforms installed in the arduino-cli data folder with their
// versions, <data>/packages/<package>/hardware/<architecture>/<version>: none when the
// folder is missing.
func (c *arduinoCLI) installedHardware() string {
	var names []string
	packages, _ := c.dataDir.Join("packages").ReadDir(paths.FilterDirectories())
	for _, pkg := range packages {
		architectures, _ := pkg.Join("hardware").ReadDir(paths.FilterDirectories())
		for _, architecture := range architectures {
			versions, _ := architecture.ReadDir(paths.FilterDirectories())
			for _, version := range versions {
				names = append(names, pkg.Base()+":"+architecture.Base()+"@"+version.Base())
			}
		}
	}
	sort.Strings(names)
	return strings.Join(names, "\n")
}

// sketchKey tells one sketch profile from another: the sketch folder and the content of its
// sketch.yaml, which names the platform and the libraries the profile resolves.
func sketchKey(sketchPath *paths.Path) (string, error) {
	project, err := sketchPath.Join("sketch.yaml").ReadFile()
	if err != nil {
		return "", fmt.Errorf("cannot read the sketch project: %w", err)
	}
	return fmt.Sprintf("%s:%x", sketchPath, sha256.Sum256(project)), nil
}

// initProgressWriter writes what an init reports to w, line by line.
func initProgressWriter(w io.Writer) func(*rpc.InitResponse) error {
	if w == nil {
		w = io.Discard
	}
	return func(r *rpc.InitResponse) error {
		var response string
		switch msg := r.GetMessage().(type) {
		case *rpc.InitResponse_InitProgress:
			if progress := msg.InitProgress.GetTaskProgress(); progress != nil {
				response = helpers.ArduinoCLITaskProgressToString(progress)
			}
			if progress := msg.InitProgress.GetDownloadProgress(); progress != nil {
				response = helpers.ArduinoCLIDownloadProgressToString(progress)
			}
		case *rpc.InitResponse_Error:
			response = "Error: " + msg.Error.String()
		case *rpc.InitResponse_Profile:
			response = fmt.Sprintf("Sketch profile configured: Name=%q, Port=%q", msg.Profile.GetName(), msg.Profile.GetPort())
		}
		_, err := w.Write([]byte(response + "\n"))
		return err
	}
}
