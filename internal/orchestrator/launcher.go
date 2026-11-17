// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/api/types/container"
	dockerClient "github.com/moby/moby/client"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/launcher"
)

// The main service of an app runs the app launcher, `/run.sh serve`: a warm interpreter that
// forks the app on request. The daemon talks to it over the socket in the runtime folder of the
// app. A launcher that refuses the start
// as stale, the dependencies changed, has its container restarted. An answer is accepted only
// from the init of the container, or its direct child under docker-init: the app shares the
// socket folder with its launcher. The launcher tells when its app starts or ends on a
// connection the status events keep open, so nothing is polled while an app runs.
const (
	// launcherSocketDir is where an app container sees the runtime folder of its app.
	launcherSocketDir = "/run/arduino-app-cli"
	// launcherAppDir is where an app container sees the app folder.
	launcherAppDir = "/app"

	// launcherRequestTimeout bounds a request the launcher answers at once.
	launcherRequestTimeout = 2 * time.Second
	// launcherStatusTimeout bounds one status request, a local socket round trip.
	launcherStatusTimeout = 500 * time.Millisecond
	// launcherStartTimeout covers a start that stops the running app first, which the
	// launcher gives 5 s before SIGKILL.
	launcherStartTimeout = 20 * time.Second
	// launcherReadyTimeout is how long a fresh container may take to answer: run.sh installs the
	// app dependencies first, which may download packages.
	launcherReadyTimeout = 10 * time.Minute
	launcherReadyPoll    = 100 * time.Millisecond
	// launcherEventsRetry is the first wait before asking again a launcher that gave no events,
	// as the one of a fresh container until it is up, and launcherEventsMaxRetry the longest.
	launcherEventsRetry    = 100 * time.Millisecond
	launcherEventsMaxRetry = time.Second
	// launcherAbortTimeout bounds the stop of the containers of an app its launcher did not start.
	launcherAbortTimeout = time.Minute

	// maxRuntimeKeyLength keeps the socket path within the Unix limit.
	maxRuntimeKeyLength = 56
	// restartGraceSeconds is what a restart of the main container gives the launcher to stop the app.
	restartGraceSeconds = 10
)

const mainServiceName = "main"

// procRoot is where the process table is read, replaced in tests.
var procRoot = "/proc"

// containerInits caches, by container ID, the host pid of the container init and whether the
// container runs an init of its own; a restarted container has a new ID.
var containerInits sync.Map

type containerInit struct {
	pid     int
	ownInit bool
}

// appRuntimeDir is the folder of the app on this board the container sees at launcherSocketDir.
func appRuntimeDir(cfg config.Configuration, appPath *paths.Path) *paths.Path {
	return cfg.RuntimeDir().Join("apps", runtimeKey(composeProjectName(appPath, cfg.AppsDir())))
}

// runtimeKey is the project name, cut and told apart by a hash when too long for a socket path.
func runtimeKey(project string) string {
	if len(project) <= maxRuntimeKeyLength {
		return project
	}
	sum := sha256.Sum256([]byte(project))
	suffix := hex.EncodeToString(sum[:6])
	return strings.TrimRight(project[:maxRuntimeKeyLength-len(suffix)-1], "-") + "-" + suffix
}

// launcherServed tells whether the project runs its main service through the app launcher:
// a release frozen before the launcher runs its app the classic way.
func launcherServed(prj *types.Project) bool {
	main, ok := prj.Services[mainServiceName]
	return ok && labeled(main.Labels, DockerAppLauncherLabel)
}

// labeled tells whether the labels carry the flag label set to true.
func labeled(labels map[string]string, label string) bool {
	return labels[label] == "true"
}

// isLauncherContainer tells a running main container whose app is driven through the launcher.
func isLauncherContainer(c container.Summary) bool {
	return labeled(c.Labels, DockerAppMainLabel) && labeled(c.Labels, DockerAppLauncherLabel) && c.State == container.StateRunning
}

// mainContainer finds the main container of the app, nil when there is none.
func mainContainer(ctx context.Context, docker dockerClient.APIClient, appPath *paths.Path) (*container.Summary, error) {
	containers, err := dockerhelper.Containers(ctx, docker, DockerAppPathLabel+"="+appPath.String())
	if err != nil {
		return nil, err
	}
	for _, c := range containers {
		if labeled(c.Labels, DockerAppMainLabel) {
			return &c, nil
		}
	}
	return nil, nil
}

// launcherClient talks to the launcher of the container and to nothing else. The container
// states where its socket is, in a label the compose template wrote.
func launcherClient(ctx context.Context, docker dockerClient.APIClient, c container.Summary) *launcher.Client {
	socket := filepath.Join(c.Labels[DockerAppRuntimeDirLabel], launcher.SocketName)
	return launcher.New(socket, launcherRequestTimeout).WithPeer(launcherCheck(ctx, docker, c.ID))
}

// launcherCheck accepts the launcher of the container only: the container init, or its direct
// child when the container runs an init of its own. Not the app, which is a child of the launcher,
// not a listener it planted on the socket path, not the launcher of another container reached
// through a link.
func launcherCheck(ctx context.Context, docker dockerClient.APIClient, containerID string) launcher.PeerCheck {
	return func(pid int) error {
		// The cache is refreshed once before refusing: the container may have been restarted
		for _, refresh := range []bool{false, true} {
			init, err := inspectInit(ctx, docker, containerID, refresh)
			if err != nil {
				return err
			}
			if pid == init.pid && !init.ownInit {
				return nil
			}
			if init.ownInit {
				parent, err := parentPID(pid)
				if err != nil {
					return err
				}
				if parent == init.pid {
					return nil
				}
			}
		}
		return fmt.Errorf("pid %d is not the launcher of the container %s", pid, containerID[:12])
	}
}

// inspectInit reads the init of the container, one inspect per container lifetime.
func inspectInit(ctx context.Context, docker dockerClient.APIClient, containerID string, refresh bool) (containerInit, error) {
	if !refresh {
		if cached, ok := containerInits.Load(containerID); ok {
			return cached.(containerInit), nil
		}
	}
	inspected, err := docker.ContainerInspect(ctx, containerID, dockerClient.ContainerInspectOptions{})
	if err != nil {
		return containerInit{}, fmt.Errorf("failed to inspect the app container: %w", err)
	}
	info := inspected.Container
	if info.State == nil || info.State.Pid == 0 {
		return containerInit{}, errors.New("the app container is not running")
	}
	init := containerInit{pid: info.State.Pid, ownInit: info.HostConfig != nil && info.HostConfig.Init != nil && *info.HostConfig.Init}
	containerInits.Store(containerID, init)
	return init, nil
}

// parentPID is the parent of the process, from its status file.
func parentPID(pid int) (int, error) {
	file, err := os.Open(filepath.Join(procRoot, strconv.Itoa(pid), "status"))
	if err != nil {
		return 0, fmt.Errorf("cannot read the process %d: %w", pid, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if value, ok := strings.CutPrefix(scanner.Text(), "PPid:"); ok {
			return strconv.Atoi(strings.TrimSpace(value))
		}
	}
	return 0, fmt.Errorf("no parent in the status of the process %d", pid)
}

// launchOrStop runs the part of a start that follows the compose up to its end, whatever its
// caller does: as a container without a launcher, which starts its app once it is up. When that
// part fails the containers of the app are stopped, so no launcher stays up without its app.
func launchOrStop(ctx context.Context, launch, stop func(context.Context) error) error {
	ctx = context.WithoutCancel(ctx)
	err := launch(ctx)
	if err == nil {
		return nil
	}
	stopCtx, cancel := context.WithTimeout(ctx, launcherAbortTimeout)
	defer cancel()
	if stopErr := stop(stopCtx); stopErr != nil {
		slog.Warn("failed to stop the app its launcher did not start", slog.String("error", stopErr.Error()))
	}
	return err
}

// startThroughLauncher starts the app in its main container, which compose brought up, through
// the launcher: a fork when the container is warm. A container whose launcher refuses the app as
// stale, its dependencies changed since it prepared them, is restarted once and asked again.
func startThroughLauncher(ctx context.Context, docker dockerClient.APIClient, appPath *paths.Path, cb func(StreamMessage)) error {
	main, err := mainContainer(ctx, docker, appPath)
	if err != nil {
		return err
	}
	if main == nil {
		return errors.New("the app container is missing after the compose up")
	}
	client := launcherClient(ctx, docker, *main)
	if err := waitLauncherReady(ctx, docker, client, main.ID); err != nil {
		return err
	}
	started := time.Now()
	resp, err := client.WithTimeout(launcherStartTimeout).Start(ctx, launcherAppDir, nil)
	if launcher.IsCode(err, launcher.CodeStale) {
		cb(StreamMessage{data: "The app dependencies changed, restarting its container"})
		if err := restartMainContainer(ctx, docker, main.ID, "the app dependencies changed"); err != nil {
			return err
		}
		if err := waitLauncherReady(ctx, docker, client, main.ID); err != nil {
			return err
		}
		resp, err = client.WithTimeout(launcherStartTimeout).Start(ctx, launcherAppDir, nil)
	}
	if err != nil {
		return fmt.Errorf("failed to start the app: %w", err)
	}
	slog.Info("app started by the launcher",
		slog.String("app", appPath.String()), slog.Int("pid", resp.PID), slog.String("runner", resp.Runner),
		slog.Float64("stop_ms", resp.Timings.StopMs), slog.Float64("fork_ms", resp.Timings.ForkMs),
		slog.Duration("elapsed", time.Since(started)))
	return nil
}

// waitLauncherReady polls status until the launcher answers, which follows the dependency install
// of a fresh container. It fails early when the container stops, and restarts the container
// once when something other than the launcher answers on the socket path.
func waitLauncherReady(ctx context.Context, docker dockerClient.APIClient, client *launcher.Client, containerID string) error {
	client = client.WithTimeout(launcherStatusTimeout)
	deadline := time.Now().Add(launcherReadyTimeout)
	restarted := false
	for {
		_, err := client.Status(ctx)
		if err == nil {
			return nil
		}
		if errors.Is(err, launcher.ErrPeer) {
			if restarted {
				return err
			}
			if err := restartMainContainer(ctx, docker, containerID, err.Error()); err != nil {
				return err
			}
			restarted = true
		}
		inspected, err := docker.ContainerInspect(ctx, containerID, dockerClient.ContainerInspectOptions{})
		if err != nil {
			return fmt.Errorf("failed to inspect the app container: %w", err)
		}
		info := inspected.Container
		if info.State == nil || !info.State.Running || info.State.Restarting {
			return fmt.Errorf("the app container stopped before its launcher answered, see `docker logs %s`", strings.TrimPrefix(info.Name, "/"))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the app launcher did not answer within %s", launcherReadyTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(launcherReadyPoll):
		}
	}
}

// restartMainContainer restarts the main container: its launcher comes back fresh, run.sh
// preparing the app again on the way.
func restartMainContainer(ctx context.Context, docker dockerClient.APIClient, containerID string, reason string) error {
	slog.Info("restarting the app container", slog.String("container", containerID[:12]), slog.String("reason", reason))
	containerInits.Delete(containerID)
	timeout := restartGraceSeconds
	if _, err := docker.ContainerRestart(ctx, containerID, dockerClient.ContainerRestartOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to restart the app container: %w", err)
	}
	return nil
}

// launcherStatus is the status of the app of a running main container driven through the
// launcher, which the state of the container does not tell: the container stays up when the
// app ends on its own. The container state stands when the launcher does not answer.
func launcherStatus(ctx context.Context, docker dockerClient.APIClient, c container.Summary) (Status, bool) {
	resp, err := launcherClient(ctx, docker, c).WithTimeout(launcherStatusTimeout).Status(ctx)
	if err != nil {
		slog.Debug("the app launcher does not answer, the container state stands", slog.String("app", c.Labels[DockerAppPathLabel]), slog.String("error", err.Error()))
		return "", false
	}
	return statusOfLaunchedApp(resp.AppState)
}

// statusOfLaunchedApp reads the status of the app from what its launcher reports, false when
// the launcher has nothing to tell: it ran no app yet. Such a launcher is met only while a
// start is in progress, since a start that fails stops the container, so the container state
// stands as while the launcher comes up. The daemon stops an app by stopping its container,
// so an app that ended in a container still up ended on its own, whatever the way: a failure,
// as for a container.
func statusOfLaunchedApp(state launcher.AppState) (Status, bool) {
	switch {
	case state.Child != nil:
		return StatusRunning, true
	case state.Last != nil:
		return StatusFailed, true
	default:
		return "", false
	}
}

// launcherStatuses asks the launcher of every running main container it drives, by container ID.
func launcherStatuses(ctx context.Context, docker dockerClient.APIClient, containers []container.Summary) map[string]Status {
	statuses := map[string]Status{}
	for _, c := range containers {
		if !isLauncherContainer(c) {
			continue
		}
		if status, ok := launcherStatus(ctx, docker, c); ok {
			statuses[c.ID] = status
		}
	}
	return statuses
}

// launcherEvents follows the app of a container: it calls onState with the state of the app
// as it is now, then at each change, until the launcher goes or the context is done.
type launcherEvents func(ctx context.Context, c container.Summary, onState func(launcher.AppState)) error

// launcherWatch keeps the events of the launcher of every running main container driven
// through one, and what each launcher last told about its app. One loop owns it: the
// followers reach it through updates only.
type launcherWatch struct {
	events  launcherEvents
	retry   time.Duration
	updates chan launcherUpdate
	// followed is by container ID.
	followed map[string]*followedLauncher
}

type followedLauncher struct {
	stop context.CancelFunc
	// status is what the launcher last told, when known.
	status Status
	known  bool
}

// launcherUpdate is the state of its app a followed launcher just told.
type launcherUpdate struct {
	containerID string
	from        *followedLauncher
	state       launcher.AppState
}

func newLauncherWatch(docker dockerClient.APIClient) *launcherWatch {
	events := func(ctx context.Context, c container.Summary, onState func(launcher.AppState)) error {
		return launcherClient(ctx, docker, c).Events(ctx, onState)
	}
	return &launcherWatch{events: events, retry: launcherEventsRetry, updates: make(chan launcherUpdate), followed: map[string]*followedLauncher{}}
}

// sync follows the launchers of the containers that run and drops the others: a container
// that stopped or restarted has nothing more to tell, its state does.
func (w *launcherWatch) sync(ctx context.Context, containers []container.Summary) {
	running := map[string]bool{}
	for _, c := range containers {
		if !isLauncherContainer(c) {
			continue
		}
		running[c.ID] = true
		if _, ok := w.followed[c.ID]; !ok {
			followCtx, stop := context.WithCancel(ctx)
			followed := &followedLauncher{stop: stop}
			w.followed[c.ID] = followed
			go w.follow(followCtx, c, followed)
		}
	}
	for id, followed := range w.followed {
		if !running[id] {
			followed.stop()
			delete(w.followed, id)
		}
	}
}

// follow sends each state the launcher tells, asking again after a wait when it gives none:
// the launcher of a fresh container answers once its dependencies are installed. What it
// last told stands when it goes, until the docker event of its container.
func (w *launcherWatch) follow(ctx context.Context, c container.Summary, from *followedLauncher) {
	wait := w.retry
	for {
		err := w.events(ctx, c, func(state launcher.AppState) {
			wait = w.retry
			select {
			case w.updates <- launcherUpdate{containerID: c.ID, from: from, state: state}:
			case <-ctx.Done():
			}
		})
		if ctx.Err() != nil {
			return
		}
		if launcher.IsCode(err, launcher.CodeBadRequest) {
			slog.Warn("the app launcher has no events, the status of its app follows the container only",
				slog.String("app", c.Labels[DockerAppPathLabel]), slog.String("error", err.Error()))
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, launcherEventsMaxRetry)
	}
}

// apply records what a launcher told. It reports false for a launcher no longer followed.
func (w *launcherWatch) apply(update launcherUpdate) bool {
	followed, ok := w.followed[update.containerID]
	if !ok || followed != update.from {
		return false
	}
	followed.status, followed.known = statusOfLaunchedApp(update.state)
	return true
}

// statuses is the status of the app each followed launcher told, by container ID.
func (w *launcherWatch) statuses() map[string]Status {
	statuses := map[string]Status{}
	for id, followed := range w.followed {
		if followed.known {
			statuses[id] = followed.status
		}
	}
	return statuses
}
