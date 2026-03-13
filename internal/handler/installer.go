package handler

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/installer"
)

// GET /tools/installer
func (h *Handler) InstallerDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	pm := installer.DetectPM()
	tools := make([]installer.ToolStatus, len(installer.Manifest))
	for i, t := range installer.Manifest {
		installed, path := installer.IsInstalled(t)
		tools[i] = installer.ToolStatus{Tool: t, Installed: installed, Path: path}
	}

	payload := map[string]any{
		"Tools": tools,
		"PM":    string(pm),
		"HasPM": pm != installer.PMNone,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "installer/dashboard.html", payload)
		return
	}
	h.render(w, "installer/dashboard.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "installer",
		ContentTpl: "installer/dashboard.html",
		Data:       payload,
	})
}

// POST /tools/installer/{name}/install
func (h *Handler) InstallTool(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	name := chi.URLParam(r, "name")
	tool := findTool(name)
	if tool == nil {
		h.operationError(w, "Ferramenta não encontrada", http.StatusNotFound)
		return
	}

	pm := installer.DetectPM()
	bin, args := installer.InstallCmd(*tool, pm)
	if bin == "" {
		h.errorToast(w, "Nenhum gerenciador de pacotes detectado")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	job := &GitOpJob{ID: newID()}
	h.installerMu.Lock()
	h.installerJobs[job.ID] = job
	h.installerMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
		h.installerMu.Lock()
		job.Output = strings.TrimSpace(string(out))
		if err != nil {
			job.Done, job.OK = true, false
			job.Error = fmt.Sprintf("Erro na instalação: %v", err)
		} else {
			job.Done, job.OK = true, true
		}
		h.installerMu.Unlock()
	}()

	h.render(w, "installer/install-progress.html", map[string]any{
		"ID": job.ID, "Done": false, "ToolName": name, "Action": "install",
	})
}

// POST /tools/installer/{name}/uninstall
func (h *Handler) UninstallTool(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	name := chi.URLParam(r, "name")
	tool := findTool(name)
	if tool == nil {
		h.operationError(w, "Ferramenta não encontrada", http.StatusNotFound)
		return
	}

	pm := installer.DetectPM()
	bin, args := installer.UninstallCmd(*tool, pm)
	if bin == "" {
		h.errorToast(w, "Nenhum gerenciador de pacotes detectado")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	job := &GitOpJob{ID: newID()}
	h.installerMu.Lock()
	h.installerJobs[job.ID] = job
	h.installerMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
		h.installerMu.Lock()
		job.Output = strings.TrimSpace(string(out))
		if err != nil {
			job.Done, job.OK = true, false
			job.Error = fmt.Sprintf("Erro na remoção: %v", err)
		} else {
			job.Done, job.OK = true, true
		}
		h.installerMu.Unlock()
	}()

	h.render(w, "installer/install-progress.html", map[string]any{
		"ID": job.ID, "Done": false, "ToolName": name, "Action": "uninstall",
	})
}

// GET /tools/installer/jobs/{id}
func (h *Handler) InstallerJobStatus(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	jobID := chi.URLParam(r, "id")
	toolName := r.URL.Query().Get("tool")
	action := r.URL.Query().Get("action")

	h.installerMu.Lock()
	job, ok := h.installerJobs[jobID]
	var done, jobOK bool
	var errMsg, output string
	if ok {
		done, jobOK = job.Done, job.OK
		errMsg, output = job.Error, job.Output
	}
	h.installerMu.Unlock()

	if !ok {
		h.render(w, "installer/install-progress.html", map[string]any{
			"Done": true, "OK": false, "ID": jobID,
			"Error": "Job não encontrado.", "ToolName": toolName, "Action": action,
		})
		return
	}
	if done {
		if jobOK {
			h.successToastOnly(w, fmt.Sprintf("Ferramenta %s: operação concluída!", toolName))
		} else {
			h.errorToast(w, errMsg)
		}
		w.WriteHeader(286) // Stop polling
	}
	h.render(w, "installer/install-progress.html", map[string]any{
		"ID":       jobID,
		"Done":     done,
		"OK":       jobOK,
		"Error":    errMsg,
		"Output":   output,
		"ToolName": toolName,
		"Action":   action,
	})
}

func findTool(name string) *installer.Tool {
	for i := range installer.Manifest {
		if installer.Manifest[i].Name == name {
			return &installer.Manifest[i]
		}
	}
	return nil
}
