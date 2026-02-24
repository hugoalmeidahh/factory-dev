package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/seuusuario/factorydev/internal/storage"
	"github.com/seuusuario/factorydev/web"
)

var tmplFuncs = template.FuncMap{
	"mb":   func(b uint64) float64 { return float64(b) / 1024 / 1024 },
	"gb":   func(b uint64) float64 { return float64(b) / 1024 / 1024 / 1024 },
	"join": func(s []string, sep string) string { return strings.Join(s, sep) },
}

type PageData struct {
	Title      string
	ActiveTool string
	ContentTpl string
	Data       any
}

func (h *Handler) render(w http.ResponseWriter, tmpl string, data any) {
	t := template.New("root").Funcs(tmplFuncs)
	var err error
	if isHXRequest(w) {
		_, err = t.ParseFS(web.FS, "templates/"+tmpl)
	} else {
		_, err = t.ParseFS(web.FS,
			"templates/layout.html",
			"templates/partials/sidebar.html",
			"templates/partials/drawer.html",
			"templates/"+tmpl,
		)
	}
	if err != nil {
		h.app.Logger.Error("erro parse template", "err", err)
		h.operationError(w, "Erro ao renderizar página", http.StatusInternalServerError)
		return
	}

	name := tmpl
	if !isHXRequest(w) {
		name = "layout.html"
	}
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		h.app.Logger.Error("erro render template", "err", err)
		h.operationError(w, "Erro ao renderizar página", http.StatusInternalServerError)
	}
}

func isHXRequest(w http.ResponseWriter) bool {
	return w.Header().Get("X-FactoryDev-HX") == "1"
}

func markHX(w http.ResponseWriter, r *http.Request) {
	if r != nil && r.Header.Get("HX-Request") == "true" {
		w.Header().Set("X-FactoryDev-HX", "1")
	}
}

func (h *Handler) renderDrawer(w http.ResponseWriter, title, tmpl string, data any) {
	contentTpl := template.New("drawer-content").Funcs(tmplFuncs)
	_, err := contentTpl.ParseFS(web.FS, "templates/"+tmpl)
	if err != nil {
		h.operationError(w, "Erro ao abrir drawer", http.StatusInternalServerError)
		return
	}
	var content bytes.Buffer
	if err := contentTpl.ExecuteTemplate(&content, tmpl, data); err != nil {
		h.operationError(w, "Erro ao abrir drawer", http.StatusInternalServerError)
		return
	}

	t := template.New("drawer-wrapper")
	_, err = t.ParseFS(web.FS, "templates/partials/drawer-wrapper.html")
	if err != nil {
		h.operationError(w, "Erro ao abrir drawer", http.StatusInternalServerError)
		return
	}

	w.Header().Set("X-FactoryDev-HX", "1")
	w.Header().Set("HX-Retarget", "#drawer-content")
	w.Header().Set("HX-Trigger", `openDrawer`)
	ctx := map[string]any{
		"Title":   title,
		"Content": template.HTML(content.String()),
	}
	if err := t.ExecuteTemplate(w, "partials/drawer-wrapper.html", ctx); err != nil {
		h.operationError(w, "Erro ao abrir drawer", http.StatusInternalServerError)
	}
}

func (h *Handler) successToast(w http.ResponseWriter, msg string) {
	trigger := fmt.Sprintf(`{"showToast":{"msg":%q,"type":"success"},"closeDrawer":true}`, msg)
	w.Header().Set("HX-Trigger", trigger)
}

func (h *Handler) successToastOnly(w http.ResponseWriter, msg string) {
	trigger := fmt.Sprintf(`{"showToast":{"msg":%q,"type":"success"}}`, msg)
	w.Header().Set("HX-Trigger", trigger)
}

func (h *Handler) repoSuccessToast(w http.ResponseWriter, msg string) {
	trigger := fmt.Sprintf(`{"showToast":{"msg":%q,"type":"success"},"closeDrawer":true}`, msg)
	w.Header().Set("HX-Trigger", trigger)
}

func (h *Handler) errorToast(w http.ResponseWriter, msg string) {
	trigger := fmt.Sprintf(`{"showToast":{"msg":%q,"type":"error"}}`, msg)
	w.Header().Set("HX-Trigger", trigger)
}

func (h *Handler) operationError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	if w.Header().Get("HX-Trigger") == "" {
		h.errorToast(w, msg)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func mapValidation(errs []storage.ValidationError) map[string]string {
	out := make(map[string]string, len(errs))
	for _, e := range errs {
		out[e.Field] = e.Message
	}
	return out
}
