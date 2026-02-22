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

type Handler struct {
	app       *app.App
	cloneJobs map[string]*CloneJob
	cloneMu   sync.Mutex
}

func New(a *app.App) *Handler {
	return &Handler{
		app:       a,
		cloneJobs: make(map[string]*CloneJob),
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
