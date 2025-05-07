package handlers

import (
	"net/http"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppCreate(dockerClient *dockerClient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		panic("not implemented")
	}
}
