package handler

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	idocker "github.com/seuusuario/factorydev/internal/docker"
)

// ContainerTemplate define um template pré-configurado de container.
type ContainerTemplate struct {
	ID          string
	Name        string
	Image       string
	Description string
	DefaultEnv  []string
	DefaultPort string
}

var containerTemplates = []ContainerTemplate{
	{ID: "postgres16", Name: "PostgreSQL 16", Image: "postgres:16-alpine",
		Description: "Banco de dados relacional PostgreSQL",
		DefaultEnv:  []string{"POSTGRES_PASSWORD=postgres", "POSTGRES_DB=app"},
		DefaultPort: "5432:5432"},
	{ID: "mysql8", Name: "MySQL 8", Image: "mysql:8",
		Description: "Banco de dados relacional MySQL",
		DefaultEnv:  []string{"MYSQL_ROOT_PASSWORD=mysql", "MYSQL_DATABASE=app"},
		DefaultPort: "3306:3306"},
	{ID: "redis7", Name: "Redis 7", Image: "redis:7-alpine",
		Description: "Cache e message broker Redis",
		DefaultEnv:  nil,
		DefaultPort: "6379:6379"},
	{ID: "mongo7", Name: "MongoDB 7", Image: "mongo:7",
		Description: "Banco de dados orientado a documentos MongoDB",
		DefaultEnv:  []string{"MONGO_INITDB_ROOT_USERNAME=root", "MONGO_INITDB_ROOT_PASSWORD=mongo"},
		DefaultPort: "27017:27017"},
	{ID: "adminer", Name: "Adminer", Image: "adminer:latest",
		Description: "Interface web para gerenciar bancos de dados",
		DefaultEnv:  nil,
		DefaultPort: "8080:8080"},
}

func newDockerClient() (*idocker.Client, error) {
	return idocker.New()
}

// GET /tools/docker
func (h *Handler) DockerDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)

	cli, err := newDockerClient()
	available := err == nil && cli.Available()

	payload := map[string]any{
		"Available": available,
	}

	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "docker/dashboard.html", payload)
		return
	}
	h.render(w, "docker/dashboard.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "docker",
		ContentTpl: "docker/dashboard.html",
		Data:       payload,
	})
}

// GET /tools/docker/status — partial com status do daemon para o header
func (h *Handler) DockerStatusPartial(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	cli, err := newDockerClient()
	if err != nil || !cli.Available() {
		h.render(w, "docker/status-header.html", map[string]any{"Available": false})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := cli.GetDaemonInfo(ctx)
	if err != nil {
		h.render(w, "docker/status-header.html", map[string]any{"Available": false})
		return
	}
	h.render(w, "docker/status-header.html", map[string]any{
		"Available": true,
		"Info":      info,
	})
}

// POST /tools/docker/start — inicia o Docker Desktop (macOS/Windows) ou mostra instrução Linux
func (h *Handler) StartDockerHandler(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	switch runtime.GOOS {
	case "darwin":
		if err := exec.Command("open", "-a", "Docker").Start(); err != nil {
			h.errorToast(w, "Não foi possível iniciar o Docker Desktop: "+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		h.successToastOnly(w, "Iniciando Docker Desktop… aguarde alguns segundos.")
	case "windows":
		// Docker Desktop no Windows
		if err := exec.Command("cmd", "/C", "start", "", "Docker Desktop").Start(); err != nil {
			h.errorToast(w, "Não foi possível iniciar o Docker Desktop.")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		h.successToastOnly(w, "Iniciando Docker Desktop…")
	default:
		h.errorToast(w, "No Linux inicie o Docker com: sudo systemctl start docker")
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
}

// GET /tools/docker/containers (partial polling)
func (h *Handler) ContainerList(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	cli, err := newDockerClient()
	if err != nil || !cli.Available() {
		h.render(w, "docker/container-list.html", map[string]any{"Available": false})
		return
	}
	containers, _ := cli.ListContainers(context.Background())
	h.render(w, "docker/container-list.html", map[string]any{
		"Available":  true,
		"Containers": containers,
	})
}

// POST /tools/docker/containers/{id}/{action}
func (h *Handler) ContainerAction(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	action := chi.URLParam(r, "action")

	cli, err := newDockerClient()
	if err != nil {
		h.errorToast(w, "Docker não disponível: "+err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := cli.Action(ctx, id, action); err != nil {
		h.errorToast(w, "Erro: "+err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	msgs := map[string]string{
		"start":   "Container iniciado!",
		"stop":    "Container parado!",
		"restart": "Container reiniciado!",
		"remove":  "Container removido!",
	}
	msg := msgs[action]
	if msg == "" {
		msg = "Ação executada!"
	}
	h.successToastOnly(w, msg)
	// trigger refresh
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast":{"msg":%q,"type":"success"},"refreshContainers":true}`, msg))
}

// GET /tools/docker/containers/{id}/logs
func (h *Handler) ContainerLogs(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	cli, err := newDockerClient()
	if err != nil {
		h.operationError(w, "Docker não disponível", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logs, err := cli.GetLogs(ctx, id, 200)
	if err != nil {
		h.operationError(w, "Erro ao buscar logs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "docker/container-detail.html", map[string]any{
		"ID":   id,
		"Logs": logs,
		"Tab":  "logs",
	})
}

// GET /tools/docker/containers/{id}
func (h *Handler) ContainerDetail(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "details"
	}
	cli, err := newDockerClient()
	if err != nil {
		h.operationError(w, "Docker não disponível", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ct, err := cli.GetContainer(ctx, id)
	if err != nil {
		h.operationError(w, "Container não encontrado: "+err.Error(), http.StatusNotFound)
		return
	}

	payload := map[string]any{
		"Container": ct,
		"Tab":       tab,
		"ID":        id,
	}

	if tab == "logs" {
		logs, _ := cli.GetLogs(ctx, id, 200)
		payload["Logs"] = logs
	}

	h.render(w, "docker/container-detail.html", payload)
}

// GET /tools/docker/images
func (h *Handler) ListImages(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	cli, err := newDockerClient()
	if err != nil || !cli.Available() {
		h.render(w, "docker/images.html", map[string]any{"Available": false})
		return
	}
	images, _ := cli.ListImages(context.Background())
	h.render(w, "docker/images.html", map[string]any{
		"Available": true,
		"Images":    images,
	})
}

// DELETE /tools/docker/images/{id}
func (h *Handler) RemoveImage(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	cli, err := newDockerClient()
	if err != nil {
		h.errorToast(w, "Docker não disponível")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cli.RemoveImage(ctx, id); err != nil {
		h.errorToast(w, "Erro ao remover imagem: "+err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToastOnly(w, "Imagem removida!")
}

// POST /tools/docker/images/pull (async)
func (h *Handler) StartPullImage(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	ref := strings.TrimSpace(r.FormValue("imageRef"))
	if ref == "" {
		h.errorToast(w, "Referência da imagem é obrigatória")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	job := &DockerJob{ID: newID()}
	h.dockerMu.Lock()
	h.dockerJobs[job.ID] = job
	h.dockerMu.Unlock()

	go func() {
		cli, err := newDockerClient()
		if err != nil {
			h.dockerMu.Lock()
			job.Done, job.OK = true, false
			job.Error = "Docker não disponível"
			h.dockerMu.Unlock()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		output, pullErr := cli.PullImage(ctx, ref)
		h.dockerMu.Lock()
		job.Output = output
		if pullErr != nil {
			job.Done, job.OK = true, false
			job.Error = pullErr.Error()
		} else {
			job.Done, job.OK = true, true
		}
		h.dockerMu.Unlock()
	}()

	h.render(w, "docker/images.html", map[string]any{
		"PullJobID": job.ID,
		"PullRef":   ref,
	})
}

// GET /tools/docker/pull-jobs/{id}
func (h *Handler) PullImageJobStatus(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	jobID := chi.URLParam(r, "id")

	h.dockerMu.Lock()
	job, ok := h.dockerJobs[jobID]
	var done, jobOK bool
	var errMsg, output string
	if ok {
		done, jobOK = job.Done, job.OK
		errMsg, output = job.Error, job.Output
	}
	h.dockerMu.Unlock()

	if !ok {
		h.render(w, "docker/images.html", map[string]any{
			"PullDone": true, "PullOK": false, "PullJobID": jobID,
			"PullError": "Job não encontrado.",
		})
		return
	}
	if done {
		if jobOK {
			w.Header().Set("HX-Trigger", `{"showToast":{"msg":"Imagem baixada com sucesso!","type":"success"},"refreshImages":true}`)
		} else {
			h.errorToast(w, errMsg)
		}
		w.WriteHeader(286)
	}
	h.render(w, "docker/images.html", map[string]any{
		"PullJobID": jobID,
		"PullDone":  done,
		"PullOK":    jobOK,
		"PullError": errMsg,
		"PullOutput": output,
	})
}

// GET /tools/docker/templates/new
func (h *Handler) TemplateDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Lançar Container", "docker/template-drawer.html", map[string]any{
		"Templates": containerTemplates,
	})
}

// POST /tools/docker/templates
func (h *Handler) LaunchTemplate(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	tmplID := strings.TrimSpace(r.FormValue("templateID"))
	name := strings.TrimSpace(r.FormValue("name"))
	imgRef := strings.TrimSpace(r.FormValue("image"))
	portStr := strings.TrimSpace(r.FormValue("port"))
	envRaw := strings.TrimSpace(r.FormValue("env"))
	restart := strings.TrimSpace(r.FormValue("restart"))

	if imgRef == "" {
		h.errorToast(w, "Imagem é obrigatória")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	if name == "" {
		name = tmplID + "-" + newID()[:8]
	}

	var envVars []string
	for _, line := range strings.Split(envRaw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			envVars = append(envVars, line)
		}
	}

	ports := map[string]string{}
	if portStr != "" {
		parts := strings.SplitN(portStr, ":", 2)
		if len(parts) == 2 {
			ports[parts[1]] = parts[0]
		}
	}

	spec := idocker.ContainerSpec{
		Name:          name,
		Image:         imgRef,
		Env:           envVars,
		Ports:         ports,
		RestartPolicy: restart,
	}

	cli, err := newDockerClient()
	if err != nil {
		h.errorToast(w, "Docker não disponível: "+err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if _, err := cli.CreateAndStart(ctx, spec); err != nil {
		h.errorToast(w, "Erro ao lançar container: "+err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.successToast(w, fmt.Sprintf("Container '%s' lançado!", name))
	h.DockerDashboard(w, r)
}
