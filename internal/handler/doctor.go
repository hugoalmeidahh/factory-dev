package handler

import (
	"net/http"

	"github.com/seuusuario/factorydev/internal/doctor"
)

func (h *Handler) Doctor(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	checks := doctor.RunDoctor(h.app.Paths)
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "doctor.html", map[string]any{"Checks": checks})
		return
	}
	h.render(w, "doctor.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "ssh",
		ContentTpl: "doctor.html",
		Data:       map[string]any{"Checks": checks},
	})
}
