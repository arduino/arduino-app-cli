package handlers

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppClone(dockerClient *dockerClient.Client) HandlerAppFunc {
	return func(w http.ResponseWriter, r *http.Request, id orchestrator.ID) {
		if id == "" {
			render.EncodeResponse(w, http.StatusPreconditionFailed, "id must be set")
			return
		}
		panic("not implemented")
	}
}
