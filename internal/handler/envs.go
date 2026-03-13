package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/storage"
)

// GET /tools/envs
func (h *Handler) ListEnvs(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	envs := make([]storage.EnvFile, len(state.EnvFiles))
	copy(envs, state.EnvFiles)
	sort.Slice(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })

	payload := map[string]any{
		"EnvFiles": envs,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "envs/list.html", payload)
		return
	}
	h.render(w, "envs/list.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "envs",
		ContentTpl: "envs/list.html",
		Data:       payload,
	})
}

// GET /tools/envs/new
func (h *Handler) NewEnvDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Novo Arquivo .env", "envs/env-drawer.html", map[string]any{
		"SubmitURL": "/tools/envs",
		"IsEdit":    false,
		"Env":       storage.EnvFile{},
		"VarsJSON":  "[]",
	})
}

// POST /tools/envs
func (h *Handler) CreateEnv(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	env := parseEnvForm(r)
	env.ID = newID()
	env.CreatedAt = time.Now()
	env.UpdatedAt = time.Now()

	if errs := storage.ValidateEnvFile(env); len(errs) > 0 {
		h.errorToast(w, errs[0].Message)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	state.EnvFiles = append(state.EnvFiles, env)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Arquivo .env criado!")
}

// GET /tools/envs/{id}/edit
func (h *Handler) EditEnvDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var found *storage.EnvFile
	for i := range state.EnvFiles {
		if state.EnvFiles[i].ID == id {
			found = &state.EnvFiles[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Arquivo .env não encontrado", http.StatusNotFound)
		return
	}

	varsJSON := envVarsToJSON(found.Variables)
	h.renderDrawer(w, "Editar .env", "envs/env-drawer.html", map[string]any{
		"SubmitURL": "/tools/envs/" + id,
		"IsEdit":    true,
		"Env":       found,
		"VarsJSON":  varsJSON,
	})
}

// POST /tools/envs/{id}
func (h *Handler) UpdateEnv(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	env := parseEnvForm(r)
	if errs := storage.ValidateEnvFile(env); len(errs) > 0 {
		h.errorToast(w, errs[0].Message)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	for i := range state.EnvFiles {
		if state.EnvFiles[i].ID == id {
			state.EnvFiles[i].Name = env.Name
			state.EnvFiles[i].ProjectPath = env.ProjectPath
			state.EnvFiles[i].Variables = env.Variables
			state.EnvFiles[i].UpdatedAt = time.Now()
			break
		}
	}
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Arquivo .env atualizado!")
}

// DELETE /tools/envs/{id}
func (h *Handler) DeleteEnv(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i, e := range state.EnvFiles {
		if e.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		h.operationError(w, "Arquivo .env não encontrado", http.StatusNotFound)
		return
	}
	state.EnvFiles = append(state.EnvFiles[:idx], state.EnvFiles[idx+1:]...)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Arquivo .env removido!")
}

// POST /tools/envs/{id}/export
func (h *Handler) ExportEnvFile(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var found *storage.EnvFile
	for i := range state.EnvFiles {
		if state.EnvFiles[i].ID == id {
			found = &state.EnvFiles[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Arquivo .env não encontrado", http.StatusNotFound)
		return
	}

	// Gera conteúdo KEY=VALUE
	var lines []string
	for k, v := range found.Variables {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(lines)
	content := strings.Join(lines, "\n") + "\n"

	// Grava em ~/.fdev/envs/<id>.env
	envsDir := h.app.Paths.Envs
	outPath := filepath.Join(envsDir, found.ID+".env")
	if err := os.WriteFile(outPath, []byte(content), 0o600); err != nil {
		h.operationError(w, "Erro ao exportar: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.successToastOnly(w, fmt.Sprintf("Exportado em %s", outPath))
}

// ── helpers ────────────────────────────────────────────────────────

func parseEnvForm(r *http.Request) storage.EnvFile {
	name := strings.TrimSpace(r.FormValue("name"))
	projectPath := strings.TrimSpace(r.FormValue("projectPath"))
	varsRaw := r.FormValue("variables")

	variables := make(map[string]string)
	if varsRaw != "" {
		var pairs []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(varsRaw), &pairs); err == nil {
			for _, p := range pairs {
				k := strings.TrimSpace(p.Key)
				if k != "" {
					variables[k] = p.Value
				}
			}
		}
	}

	return storage.EnvFile{
		Name:        name,
		ProjectPath: projectPath,
		Variables:   variables,
	}
}

func envVarsToJSON(vars map[string]string) string {
	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	pairs := make([]kv, 0, len(vars))
	for k, v := range vars {
		pairs = append(pairs, kv{Key: k, Value: v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
	b, _ := json.Marshal(pairs)
	return string(b)
}
