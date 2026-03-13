package handler

import (
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-chi/chi/v5"
)

type processView struct {
	PID    int32
	Name   string
	CPU    float64
	Mem    float32
	User   string
	Status string
}

// GET /tools/tasks
func (h *Handler) TasksDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	procs := getTopProcesses(50)
	payload := map[string]any{
		"Processes": procs,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "tasks/dashboard.html", payload)
		return
	}
	h.render(w, "tasks/dashboard.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "tasks",
		ContentTpl: "tasks/dashboard.html",
		Data:       payload,
	})
}

// GET /tools/tasks/list
func (h *Handler) TasksListPartial(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	procs := getTopProcesses(50)
	h.render(w, "tasks/list.html", map[string]any{
		"Processes": procs,
	})
}

// POST /tools/tasks/{pid}/kill
func (h *Handler) KillProcess(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	pidStr := chi.URLParam(r, "pid")
	pid, err := strconv.ParseInt(pidStr, 10, 32)
	if err != nil {
		h.errorToast(w, "PID inválido")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := syscall.Kill(int(pid), syscall.SIGTERM); err != nil {
		h.errorToast(w, fmt.Sprintf("Erro ao encerrar PID %d: %v", pid, err))
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToastOnly(w, fmt.Sprintf("Sinal SIGTERM enviado ao PID %d", pid))
}

// getTopProcesses usa `ps` nativo para listar processos — rápido em qualquer OS.
func getTopProcesses(limit int) []processView {
	// ps -eo pid,pcpu,pmem,user,stat,comm — funciona em macOS e Linux
	out, err := exec.Command("ps", "-eo", "pid,pcpu,pmem,user,stat,comm").Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil
	}

	var views []processView
	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		pid, _ := strconv.ParseInt(fields[0], 10, 32)
		cpu, _ := strconv.ParseFloat(fields[1], 64)
		mem, _ := strconv.ParseFloat(fields[2], 64)
		user := fields[3]
		status := fields[4]
		name := strings.Join(fields[5:], " ")

		views = append(views, processView{
			PID:    int32(pid),
			Name:   name,
			CPU:    cpu,
			Mem:    float32(mem),
			User:   user,
			Status: status,
		})
	}

	sort.Slice(views, func(i, j int) bool { return views[i].CPU > views[j].CPU })
	if len(views) > limit {
		views = views[:limit]
	}
	return views
}
