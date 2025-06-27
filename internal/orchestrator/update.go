package orchestrator

import (
	"bufio"
	"context"
	"errors"
	"io"
	"iter"
	"regexp"
	"strings"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/pkg/x"
)

var ErrNoUpgradablePackages = errors.New("no upgradable packages found")

type UpgradablePackage struct {
	Name         string `json:"name"` // Package name without repository information
	Architecture string `json:"-"`
	FromVersion  string `json:"from_version"`
	ToVersion    string `json:"to_version"`
}

func RunUpgradeCommand(ctx context.Context, pks []UpgradablePackage) (iter.Seq[string], error) {
	if len(pks) == 0 {
		return x.EmptyIter[string](), ErrNoUpgradablePackages
	}
	names := f.Map(pks, func(p UpgradablePackage) string {
		return p.Name
	})
	args := append([]string{"sudo", "apt-get", "upgrade", "-y"}, names...)
	upgradeCmd, err := paths.NewProcess([]string{"NEEDRESTART_MODE=l"}, args...)
	if err != nil {
		return nil, err
	}

	return func(yield func(string) bool) {
		stdout := NewCallbackWriter(func(line string) {
			if !yield(line) {
				return
			}
		})
		upgradeCmd.RedirectStdoutTo(stdout)
		if err := upgradeCmd.RunWithinContext(ctx); err != nil {
			return
		}
	}, nil

}

// RestartServices restarts services that need to be restarted after an upgrade.
// It uses the `needrestart` command to determine which services need to be restarted.
// It returns an error if the command fails to start or if it fails to wait for the command to finish.
// It uses the '-r a' option to restart all services that need to be restarted automatically without prompting the user
// Note: This function does not take the list of services as an argument because
// `needrestart` automatically detects which services need to be restarted based on the system state.
func RestartServices(ctx context.Context) error {
	needRestartCmd, err := paths.NewProcess(nil, "sudo", "needrestart", "-r", "a")
	if err != nil {
		return err
	}
	err = needRestartCmd.RunWithinContext(ctx)
	if err != nil {
		return err
	}
	return nil
}

func GetUpgradablePackages(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error) {
	updateCmd, err := paths.NewProcess(nil, "sudo", "apt-get", "update")
	if err != nil {
		return nil, err
	}
	err = updateCmd.Run()
	if err != nil {
		return nil, err
	}

	listUpgradable, err := paths.NewProcess(nil, "apt", "list", "--upgradable")
	if err != nil {
		return nil, err
	}

	out, err := listUpgradable.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = listUpgradable.Start()
	if err != nil {
		return nil, err
	}

	packages := parseListUpgradableOutput(out)

	if err := listUpgradable.Wait(); err != nil {
		return nil, err
	}

	filtered := f.Filter(packages, matcher)

	return filtered, nil
}

// parseListUpgradableOutput parses the output of `apt list --upgradable` command
// Example: apt/focal-updates 2.0.11 amd64 [upgradable from: 2.0.10]
func parseListUpgradableOutput(r io.Reader) []UpgradablePackage {
	re := regexp.MustCompile(`^([^ ]+) ([^ ]+) ([^ ]+)(?: \[upgradable from: ([^\[\]]*)\])?`)

	res := []UpgradablePackage{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		matches := re.FindStringSubmatch(scanner.Text())
		if len(matches) == 0 {
			continue
		}

		// Remove repository information in name
		// example: "libgweather-common/zesty-updates,zesty-updates"
		//       -> "libgweather-common"
		name := strings.Split(matches[1], "/")[0]

		pkg := UpgradablePackage{
			Name:         name,
			ToVersion:    matches[2],
			Architecture: matches[3],
			FromVersion:  matches[4],
		}
		res = append(res, pkg)
	}
	return res
}
