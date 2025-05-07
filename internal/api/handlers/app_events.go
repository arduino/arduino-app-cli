package handlers

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppEvents(dockerClient *dockerClient.Client) HandlerAppFunc {
	return func(w http.ResponseWriter, r *http.Request, id orchestrator.ID) {
		panic("not implemented")
	}
}
