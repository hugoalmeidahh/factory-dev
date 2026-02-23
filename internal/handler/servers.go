package handler

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/app"
	igit "github.com/seuusuario/factorydev/internal/git"
	"github.com/seuusuario/factorydev/internal/storage"
)

type serverView struct {
	storage.Server
	KeyName string
	HasKey  bool
}

// GET /tools/servers
func (h *Handler) ListServers(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	keyMap := make(map[string]storage.Key, len(state.Keys))
	for _, k := range state.Keys {
		keyMap[k.ID] = k
	}

	views := make([]serverView, len(state.Servers))
	for i, s := range state.Servers {
		v := serverView{Server: s}
		if k, ok := keyMap[s.KeyID]; ok {
			v.KeyName = k.Name
			v.HasKey = true
		}
		views[i] = v
	}

	payload := map[string]any{
		"Servers": views,
		"Keys":    state.Keys,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "servers/list.html", payload)
		return
	}
	h.render(w, "servers/list.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "servers",
		ContentTpl: "servers/list.html",
		Data:       payload,
	})
}

// GET /tools/servers/new
func (h *Handler) NewServerDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.renderDrawer(w, "Novo Servidor", "servers/server-drawer.html", map[string]any{
		"Keys":      state.Keys,
		"SubmitURL": "/tools/servers",
		"IsEdit":    false,
		"Server":    storage.Server{Port: 22},
	})
}

// POST /tools/servers
func (h *Handler) CreateServer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	srv, errMsg := parseServerForm(r)
	if errMsg != "" {
		h.errorToast(w, errMsg)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	srv.ID = newID()
	srv.CreatedAt = time.Now()

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	state.Servers = append(state.Servers, srv)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Servidor criado!")
	h.ListServers(w, r)
}

// GET /tools/servers/{id}/edit
func (h *Handler) EditServerDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var found *storage.Server
	for i := range state.Servers {
		if state.Servers[i].ID == id {
			found = &state.Servers[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Servidor não encontrado", http.StatusNotFound)
		return
	}
	h.renderDrawer(w, "Editar Servidor", "servers/server-drawer.html", map[string]any{
		"Keys":      state.Keys,
		"SubmitURL": "/tools/servers/" + id,
		"IsEdit":    true,
		"Server":    found,
	})
}

// POST /tools/servers/{id}
func (h *Handler) UpdateServer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	srv, errMsg := parseServerForm(r)
	if errMsg != "" {
		h.errorToast(w, errMsg)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	for i := range state.Servers {
		if state.Servers[i].ID == id {
			state.Servers[i].Name = srv.Name
			state.Servers[i].Host = srv.Host
			state.Servers[i].Port = srv.Port
			state.Servers[i].User = srv.User
			state.Servers[i].KeyID = srv.KeyID
			state.Servers[i].Description = srv.Description
			state.Servers[i].Tags = srv.Tags
			break
		}
	}
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Servidor atualizado!")
	h.ListServers(w, r)
}

// DELETE /tools/servers/{id}
func (h *Handler) DeleteServer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i, s := range state.Servers {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		h.operationError(w, "Servidor não encontrado", http.StatusNotFound)
		return
	}
	state.Servers = append(state.Servers[:idx], state.Servers[idx+1:]...)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Servidor removido!")
	h.ListServers(w, r)
}

// POST /tools/servers/{id}/test
func (h *Handler) StartTestJob(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	srv, keyPath := findServerAndKey(state, id)
	if srv == nil {
		h.operationError(w, "Servidor não encontrado", http.StatusNotFound)
		return
	}

	job := &GitOpJob{ID: newID()}
	h.serverTestMu.Lock()
	h.serverTestJobs[job.ID] = job
	h.serverTestMu.Unlock()

	port := srv.Port
	if port == 0 {
		port = 22
	}
	user := srv.User
	host := srv.Host
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		args := []string{
			"-o", "ConnectTimeout=5",
			"-o", "BatchMode=yes",
			"-o", "StrictHostKeyChecking=accept-new",
			"-p", strconv.Itoa(port),
		}
		if keyPath != "" {
			args = append(args, "-i", keyPath)
		}
		args = append(args, user+"@"+host, "exit")
		out, sshErr := exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
		h.serverTestMu.Lock()
		job.Output = strings.TrimSpace(string(out))
		if sshErr != nil {
			job.Done, job.OK = true, false
			job.Error = fmt.Sprintf("Falha na conexão: %v", sshErr)
		} else {
			job.Done, job.OK = true, true
		}
		h.serverTestMu.Unlock()
	}()

	h.render(w, "servers/test-progress.html", map[string]any{
		"ID": job.ID, "Done": false, "ServerID": id,
	})
}

// GET /tools/servers/test-jobs/{jobId}
func (h *Handler) TestJobStatus(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	jobID := chi.URLParam(r, "jobId")
	serverID := r.URL.Query().Get("serverId")

	h.serverTestMu.Lock()
	job, ok := h.serverTestJobs[jobID]
	var done, jobOK bool
	var errMsg, output string
	if ok {
		done, jobOK = job.Done, job.OK
		errMsg, output = job.Error, job.Output
	}
	h.serverTestMu.Unlock()

	if !ok {
		h.render(w, "servers/test-progress.html", map[string]any{
			"Done": true, "OK": false, "ID": jobID, "ServerID": serverID,
			"Error": "Job não encontrado.",
		})
		return
	}
	if done {
		if jobOK {
			w.Header().Set("HX-Trigger", `{"showToast":{"msg":"Conexão SSH bem-sucedida!","type":"success"}}`)
		} else {
			h.errorToast(w, errMsg)
		}
		w.WriteHeader(286)
	}
	h.render(w, "servers/test-progress.html", map[string]any{
		"ID":       jobID,
		"ServerID": serverID,
		"Done":     done,
		"OK":       jobOK,
		"Error":    errMsg,
		"Output":   output,
	})
}

// POST /tools/servers/{id}/connect
func (h *Handler) ConnectServer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	srv, keyPath := findServerAndKey(state, id)
	if srv == nil {
		h.operationError(w, "Servidor não encontrado", http.StatusNotFound)
		return
	}

	port := srv.Port
	if port == 0 {
		port = 22
	}
	sshArgs := []string{
		"-p", strconv.Itoa(port),
		"-o", "StrictHostKeyChecking=accept-new",
	}
	if keyPath != "" {
		sshArgs = append(sshArgs, "-i", keyPath)
	}
	sshArgs = append(sshArgs, srv.User+"@"+srv.Host)

	if err := igit.OpenTerminalWithCmd("ssh", sshArgs...); err != nil {
		h.errorToast(w, err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToastOnly(w, "Terminal SSH aberto!")
}

// GET /tools/servers/{id}/send-file
func (h *Handler) SendFileDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var found *storage.Server
	for i := range state.Servers {
		if state.Servers[i].ID == id {
			found = &state.Servers[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Servidor não encontrado", http.StatusNotFound)
		return
	}
	h.renderDrawer(w, "Enviar Arquivo", "servers/send-file-drawer.html", map[string]any{
		"Server": found,
	})
}

// POST /tools/servers/{id}/send-file
func (h *Handler) StartSendFileJob(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	localPath := strings.TrimSpace(r.FormValue("localPath"))
	remoteDest := strings.TrimSpace(r.FormValue("remoteDest"))
	if localPath == "" || remoteDest == "" {
		h.errorToast(w, "Caminho local e destino remoto são obrigatórios")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	srv, keyPath := findServerAndKey(state, id)
	if srv == nil {
		h.operationError(w, "Servidor não encontrado", http.StatusNotFound)
		return
	}

	job := &GitOpJob{ID: newID()}
	h.sendFileMu.Lock()
	h.sendFileJobs[job.ID] = job
	h.sendFileMu.Unlock()

	port := srv.Port
	if port == 0 {
		port = 22
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		args := []string{"-P", strconv.Itoa(port), "-o", "StrictHostKeyChecking=accept-new"}
		if keyPath != "" {
			args = append(args, "-i", keyPath)
		}
		args = append(args, localPath, srv.User+"@"+srv.Host+":"+remoteDest)
		out, scpErr := exec.CommandContext(ctx, "scp", args...).CombinedOutput()
		h.sendFileMu.Lock()
		job.Output = strings.TrimSpace(string(out))
		if scpErr != nil {
			job.Done, job.OK = true, false
			job.Error = fmt.Sprintf("Erro no envio: %v", scpErr)
		} else {
			job.Done, job.OK = true, true
		}
		h.sendFileMu.Unlock()
	}()

	h.render(w, "servers/test-progress.html", map[string]any{
		"ID": job.ID, "Done": false, "ServerID": id, "IsSend": true,
	})
}

// GET /tools/servers/send-jobs/{jobId}
func (h *Handler) SendFileJobStatus(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	jobID := chi.URLParam(r, "jobId")
	serverID := r.URL.Query().Get("serverId")

	h.sendFileMu.Lock()
	job, ok := h.sendFileJobs[jobID]
	var done, jobOK bool
	var errMsg, output string
	if ok {
		done, jobOK = job.Done, job.OK
		errMsg, output = job.Error, job.Output
	}
	h.sendFileMu.Unlock()

	if !ok {
		h.render(w, "servers/test-progress.html", map[string]any{
			"Done": true, "OK": false, "ID": jobID, "ServerID": serverID, "IsSend": true,
			"Error": "Job não encontrado.",
		})
		return
	}
	if done {
		if jobOK {
			w.Header().Set("HX-Trigger", `{"showToast":{"msg":"Arquivo enviado com sucesso!","type":"success"},"closeDrawer":true}`)
		} else {
			h.errorToast(w, errMsg)
		}
		w.WriteHeader(286)
	}
	h.render(w, "servers/test-progress.html", map[string]any{
		"ID":       jobID,
		"ServerID": serverID,
		"IsSend":   true,
		"Done":     done,
		"OK":       jobOK,
		"Error":    errMsg,
		"Output":   output,
	})
}

// ── helpers ────────────────────────────────────────────────────────

func parseServerForm(r *http.Request) (storage.Server, string) {
	name := strings.TrimSpace(r.FormValue("name"))
	host := strings.TrimSpace(r.FormValue("host"))
	user := strings.TrimSpace(r.FormValue("user"))
	portStr := strings.TrimSpace(r.FormValue("port"))
	keyID := strings.TrimSpace(r.FormValue("keyID"))
	desc := strings.TrimSpace(r.FormValue("description"))
	tagsRaw := strings.TrimSpace(r.FormValue("tags"))

	if name == "" || host == "" || user == "" {
		return storage.Server{}, "Nome, host e usuário são obrigatórios"
	}
	port := 22
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			return storage.Server{}, "Porta inválida"
		}
		port = p
	}

	var tags []string
	for _, t := range strings.Split(tagsRaw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}

	return storage.Server{
		Name:        name,
		Host:        host,
		Port:        port,
		User:        user,
		KeyID:       keyID,
		Description: desc,
		Tags:        tags,
	}, ""
}

func findServerAndKey(state *storage.State, serverID string) (*storage.Server, string) {
	var srv *storage.Server
	for i := range state.Servers {
		if state.Servers[i].ID == serverID {
			srv = &state.Servers[i]
			break
		}
	}
	if srv == nil {
		return nil, ""
	}
	var keyPath string
	for _, k := range state.Keys {
		if k.ID == srv.KeyID {
			keyPath = k.PrivateKeyPath
			break
		}
	}
	return srv, keyPath
}
