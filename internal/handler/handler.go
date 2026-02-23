package handler

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"

	"github.com/seuusuario/factorydev/internal/app"
)

// CloneJob representa um clone de repositório em andamento ou concluído.
type CloneJob struct {
	ID     string
	Done   bool
	OK     bool
	Output string
	Error  string
}

// GitOpJob representa uma operação git genérica assíncrona (pull, test SSH, send-file, etc).
type GitOpJob struct {
	ID     string
	Done   bool
	OK     bool
	Output string
	Error  string
}

// DockerJob representa uma operação Docker assíncrona (pull de imagem, etc).
type DockerJob struct {
	ID     string
	Done   bool
	OK     bool
	Output string
	Error  string
}

type Handler struct {
	app       *app.App
	cloneJobs map[string]*CloneJob
	cloneMu   sync.Mutex
	// Pull jobs
	pullJobs map[string]*GitOpJob
	pullMu   sync.Mutex
	// Server test/connect jobs
	serverTestJobs map[string]*GitOpJob
	serverTestMu   sync.Mutex
	// Send file jobs
	sendFileJobs map[string]*GitOpJob
	sendFileMu   sync.Mutex
	// Docker jobs (pull image)
	dockerJobs map[string]*DockerJob
	dockerMu   sync.Mutex
}

func New(a *app.App) *Handler {
	return &Handler{
		app:            a,
		cloneJobs:      make(map[string]*CloneJob),
		pullJobs:       make(map[string]*GitOpJob),
		serverTestJobs: make(map[string]*GitOpJob),
		sendFileJobs:   make(map[string]*GitOpJob),
		dockerJobs:     make(map[string]*DockerJob),
	}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.ListAccounts(w, r)
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				h.app.Logger.Error("panic capturado", "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
				h.operationError(w, "Ocorreu um erro inesperado. Verifique os logs.", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
