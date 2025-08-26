package orchestrator

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestParseAppStatus(t *testing.T) {
	tests := []struct {
		name           string
		containerState []container.ContainerState
		want           Status
	}{
		{
			name:           "everything running",
			containerState: []container.ContainerState{container.StateRunning, container.StateRunning},
			want:           StatusRunning,
		},
		{
			name:           "everything stopped",
			containerState: []container.ContainerState{container.StateCreated, container.StatePaused, container.StateExited},
			want:           StatusStopped,
		},
		{
			name:           "failed container",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead},
			want:           StatusFailed,
		},
		{
			name:           "failed container takes precedence over stopping and starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead, container.StateRemoving, container.StateRestarting},
			want:           StatusFailed,
		},
		{
			name:           "stopping",
			containerState: []container.ContainerState{container.StateRunning, container.StateRemoving},
			want:           StatusStopping,
		},
		{
			name:           "stopping takes precedence over starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateRestarting, container.StateRemoving},
			want:           StatusStopping,
		},
		{
			name:           "starting",
			containerState: []container.ContainerState{container.StateRestarting, container.StateExited},
			want:           StatusStarting,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := f.Map(tc.containerState, func(c container.ContainerState) container.Summary {
				return container.Summary{
					Labels: map[string]string{DockerAppPathLabel: "path1"},
					State:  c,
				}
			})
			res := parseAppStatus(input)
			require.Len(t, res, 1)
			require.Equal(t, tc.want, res[0].Status)
			require.Equal(t, "path1", res[0].AppPath.String())
		})
	}
}
