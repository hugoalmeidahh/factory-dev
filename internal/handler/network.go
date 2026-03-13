package handler

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type connectionView struct {
	PID        int32
	ProcName   string
	LocalAddr  string
	RemoteAddr string
	Status     string
}

// GET /tools/network
func (h *Handler) NetworkDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	conns := getConnections()
	payload := map[string]any{
		"Connections": conns,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "network/dashboard.html", payload)
		return
	}
	h.render(w, "network/dashboard.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "network",
		ContentTpl: "network/dashboard.html",
		Data:       payload,
	})
}

// GET /tools/network/list
func (h *Handler) NetworkListPartial(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	conns := getConnections()
	h.render(w, "network/list.html", map[string]any{
		"Connections": conns,
	})
}

// POST /tools/network/block
func (h *Handler) BlockConnection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	ip := strings.TrimSpace(r.FormValue("ip"))
	if ip == "" {
		h.errorToast(w, "IP é obrigatório")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("sudo", "pfctl", "-t", "fdev_blocked", "-T", "add", ip)
	} else {
		cmd = exec.Command("sudo", "ip", "route", "add", "blackhole", ip)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		h.errorToast(w, fmt.Sprintf("Erro ao bloquear %s: %v (%s)", ip, err, strings.TrimSpace(string(out))))
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToastOnly(w, fmt.Sprintf("IP %s bloqueado!", ip))
}

// getConnections usa lsof/ss nativo para listar conexões TCP — rápido.
func getConnections() []connectionView {
	var out []byte
	var err error

	if runtime.GOOS == "darwin" {
		out, err = exec.Command("lsof", "-i", "tcp", "-n", "-P", "-sTCP:ESTABLISHED").Output()
	} else {
		out, err = exec.Command("ss", "-tnp").Output()
	}
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil
	}

	var views []connectionView
	if runtime.GOOS == "darwin" {
		views = parseLsofOutput(lines[1:])
	} else {
		views = parseSSOutput(lines[1:])
	}

	sort.Slice(views, func(i, j int) bool { return views[i].ProcName < views[j].ProcName })
	if len(views) > 100 {
		views = views[:100]
	}
	return views
}

func parseLsofOutput(lines []string) []connectionView {
	var views []connectionView
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		procName := fields[0]
		pid, _ := strconv.ParseInt(fields[1], 10, 32)
		// campo 8 = NAME: local->remote
		nameParts := strings.SplitN(fields[8], "->", 2)
		if len(nameParts) != 2 {
			continue
		}
		views = append(views, connectionView{
			PID:        int32(pid),
			ProcName:   procName,
			LocalAddr:  nameParts[0],
			RemoteAddr: nameParts[1],
			Status:     "ESTABLISHED",
		})
	}
	return views
}

func parseSSOutput(lines []string) []connectionView {
	var views []connectionView
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		state := fields[0]
		localAddr := fields[3]
		remoteAddr := fields[4]

		var procName string
		var pid int32
		if len(fields) >= 6 {
			p := fields[5]
			if idx := strings.Index(p, "pid="); idx >= 0 {
				rest := p[idx+4:]
				if comma := strings.Index(rest, ","); comma >= 0 {
					n, _ := strconv.ParseInt(rest[:comma], 10, 32)
					pid = int32(n)
				}
			}
			if idx := strings.Index(p, "((\""); idx >= 0 {
				rest := p[idx+3:]
				if quote := strings.Index(rest, "\""); quote >= 0 {
					procName = rest[:quote]
				}
			}
		}

		views = append(views, connectionView{
			PID:        pid,
			ProcName:   procName,
			LocalAddr:  localAddr,
			RemoteAddr: remoteAddr,
			Status:     state,
		})
	}
	return views
}
