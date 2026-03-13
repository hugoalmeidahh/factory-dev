package handler

import (
	"net/http"
	"strings"

	"github.com/seuusuario/factorydev/internal/firewall"
)

// GET /tools/firewall
func (h *Handler) FirewallDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	status := firewall.GetStatus()
	payload := map[string]any{
		"Status": status,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "firewall/dashboard.html", payload)
		return
	}
	h.render(w, "firewall/dashboard.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "firewall",
		ContentTpl: "firewall/dashboard.html",
		Data:       payload,
	})
}

// POST /tools/firewall/toggle
func (h *Handler) ToggleFirewall(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	enable := r.FormValue("enable") == "true"
	if err := firewall.Toggle(enable); err != nil {
		h.errorToast(w, "Erro: "+err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	if enable {
		h.successToastOnly(w, "Firewall ativado!")
	} else {
		h.successToastOnly(w, "Firewall desativado!")
	}
}

// POST /tools/firewall/rules
func (h *Handler) AddFirewallRule(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	port := strings.TrimSpace(r.FormValue("port"))
	proto := strings.TrimSpace(r.FormValue("proto"))
	action := strings.TrimSpace(r.FormValue("action"))

	if port == "" || proto == "" {
		h.errorToast(w, "Porta e protocolo são obrigatórios")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	var err error
	if action == "deny" {
		err = firewall.DenyPort(port, proto)
	} else {
		err = firewall.AllowPort(port, proto)
	}
	if err != nil {
		h.errorToast(w, "Erro: "+err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToastOnly(w, "Regra adicionada!")
}
