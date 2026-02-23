package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/system"
)

// GET /tools/system
func (h *Handler) SystemDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	info, err := system.Gather(context.Background())
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	payload := map[string]any{"Info": info}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "system/dashboard.html", payload)
		return
	}
	h.render(w, "system/dashboard.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "system",
		ContentTpl: "system/dashboard.html",
		Data:       payload,
	})
}

// GET /tools/system/widgets
func (h *Handler) SystemWidgets(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	info, _ := system.Gather(context.Background())
	h.render(w, "system/widgets.html", map[string]any{"Info": info})
}

// POST /tools/system/hostname
func (h *Handler) SetHostname(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("hostname"))
	if name == "" {
		h.errorToast(w, "Hostname não pode ser vazio")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	if err := system.SetHostname(name); err != nil {
		h.errorToast(w, err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToastOnly(w, "Hostname atualizado para: "+name)
}
