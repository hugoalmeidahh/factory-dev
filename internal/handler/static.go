package handler

import (
	"io/fs"
	"net/http"

	"github.com/seuusuario/factorydev/web"
)

func (h *Handler) staticHandler() http.Handler {
	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(staticFS))
}
