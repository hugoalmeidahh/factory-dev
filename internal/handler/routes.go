package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(h.recoverer)

	r.Get("/", h.Index)
	r.Get("/health", h.Health)
	r.Get("/doctor", h.Doctor)
	r.Handle("/assets/*", http.StripPrefix("/assets/", h.staticHandler()))

	// API utilitários
	r.Get("/api/scan-summary", h.ScanSummary)

	// SSH Accounts (modo completo)
	r.Get("/tools/ssh/accounts", h.ListAccounts)
	r.Post("/tools/ssh/accounts", h.CreateAccount)
	r.Get("/tools/ssh/accounts/new", h.NewAccountDrawer)
	r.Get("/tools/ssh/accounts/{id}/edit", h.EditAccountDrawer)
	r.Post("/tools/ssh/accounts/{id}", h.UpdateAccount)
	r.Delete("/tools/ssh/accounts/{id}", h.DeleteAccount)
	r.Post("/tools/ssh/accounts/{id}/generate-key", h.GenerateKey)
	r.Post("/tools/ssh/accounts/{id}/apply-ssh", h.ApplySSHConfig)
	r.Post("/tools/ssh/accounts/{id}/test", h.TestConnection)
	r.Post("/tools/ssh/accounts/{id}/preview-apply", h.PreviewApplySSHConfig)

	// SSH Keys (modo simples)
	r.Get("/tools/ssh/keys/new", h.NewSimpleKeyDrawer)
	r.Post("/tools/ssh/keys", h.CreateSimpleKey)

	// Repositórios
	r.Get("/tools/repos", h.Repositories)
	r.Get("/tools/repos/clone/new", h.NewCloneDrawer)
	r.Post("/tools/repos/clone", h.StartCloneJob)
	r.Get("/tools/repos/jobs/{id}", h.CloneJobStatus)
	r.Delete("/tools/repos/{id}", h.DeleteRepository)
	r.Get("/tools/repos/{id}/status", h.RepoStatus)

	return r
}
