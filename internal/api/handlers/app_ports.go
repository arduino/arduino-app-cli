package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppPortResponse struct {
	Ports []port `json:"ports" example:"80" description:"exposed port of the app"`
}
type port struct {
	Port        string `json:"port" example:"80" description:"exposed port	of the app"`
	Source      string `json:"source" example:"brick:data-storage" description:"source of the port, e.g. app or brick:data-storage"`
	ServiceName string `json:"serviceName,omitempty" example:"Web Interface" description:"name of the service if the port is exposed by a brick"`
}

func HandleAppPorts(
	bricksIndex *bricksindex.BricksIndex,
	idProvider *app.IDProvider,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		app, err := app.Load(id.ToPath().String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}
		brickInfoMap, err := GetBrickPortInfoByID(app.Descriptor.Bricks, bricksIndex)
		if err != nil {
			slog.Error("Unable to find bricks ports", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Unable to find bricks ports"})
			return
		}
		response := buildAppPortResponse(app.Descriptor.Ports, brickInfoMap)

		render.EncodeResponse(w, http.StatusOK, response)
	}
}

func buildAppPortResponse(appPorts []int, brickInfoMap map[string]BrickPortInfo) AppPortResponse {
	response := AppPortResponse{
		Ports: make([]port, 0, len(appPorts)+len(brickInfoMap)),
	}

	for _, p := range appPorts {
		response.Ports = append(response.Ports, port{
			Port:        strconv.Itoa(p),
			Source:      "app.yaml",
			ServiceName: "webview",
		})
	}

	for source, brickInfo := range brickInfoMap {
		for _, p := range brickInfo.Ports {
			response.Ports = append(response.Ports, port{
				Port:        p,
				Source:      source,
				ServiceName: brickInfo.RequiresDisplay,
			})
		}
	}

	return response
}

type BrickPortInfo struct {
	Ports           []string
	RequiresDisplay string
}

func GetBrickPortInfoByID(bricks []app.Brick, bricksIndex *bricksindex.BricksIndex) (map[string]BrickPortInfo, error) {

	brickInfoByID := make(map[string]BrickPortInfo)

	for _, brick := range bricks {
		brickData, found := bricksIndex.FindBrickByID(brick.ID)
		if !found {
			return nil, fmt.Errorf("brick %q not found in the index", brick.ID)
		}
		brickInfoByID[brick.ID] = BrickPortInfo{
			Ports:           brickData.Ports,
			RequiresDisplay: brickData.RequiresDisplay,
		}
	}

	return brickInfoByID, nil
}
