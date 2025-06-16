package handlers

import (
	"io/fs"
	"net/http"
)

func DocsServer(contentFS fs.FS) http.Handler {
	subFS, err := fs.Sub(contentFS, "docs")
	if err != nil {
		panic("embedded docs folder not found" + err.Error())
	}
	return http.FileServer(http.FS(subFS))
}
