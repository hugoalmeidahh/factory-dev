package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/storage"
)

type reposAccountView struct {
	ID        string
	Name      string
	HostAlias string
	HostName  string
	HasKey    bool
}

func (h *Handler) Repositories(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	accounts, err := h.repoAccounts()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	payload := map[string]any{
		"Accounts": accounts,
	}

	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "repos/list.html", payload)
		return
	}
	h.render(w, "repos/list.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "repos",
		ContentTpl: "repos/list.html",
		Data:       payload,
	})
}

func (h *Handler) NewCloneDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	accounts, err := h.repoAccounts()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	defaultDest := filepath.Join(h.app.Paths.Home, "workspace", "repo")
	h.renderDrawer(w, "Clonar Repositório", "repos/clone-drawer.html", map[string]any{
		"Accounts":    accounts,
		"DefaultDest": defaultDest,
	})
}

func (h *Handler) CloneRepository(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	accountID := strings.TrimSpace(r.FormValue("accountID"))
	rawURL := strings.TrimSpace(r.FormValue("repoURL"))
	destDir := strings.TrimSpace(r.FormValue("destDir"))

	if accountID == "" || rawURL == "" || destDir == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.render(w, "repos/clone-result.html", map[string]any{
			"OK":      false,
			"Message": "Preencha conta, URL e diretório de destino.",
		})
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	account, found := findAccountByID(state.Accounts, accountID)
	if !found {
		h.operationError(w, "Conta não encontrada", http.StatusNotFound)
		return
	}

	identityPath := h.app.Paths.PrivateKey(account.HostAlias)
	if _, err := os.Stat(identityPath); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.render(w, "repos/clone-result.html", map[string]any{
			"OK":      false,
			"Message": "A conta selecionada não possui chave privada. Gere a chave primeiro.",
		})
		return
	}

	sshURL, err := h.app.GitService.BuildSSHURL(rawURL, account.HostAlias)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.render(w, "repos/clone-result.html", map[string]any{
			"OK":      false,
			"Message": "URL de repositório inválida: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	output, cloneErr := h.app.GitService.CloneRepo(ctx, sshURL, destDir, identityPath)
	if cloneErr != nil {
		h.app.Logger.Error("falha no clone do repositório",
			"account_id", accountID,
			"host_alias", account.HostAlias,
			"repo_url", rawURL,
			"ssh_url", sshURL,
			"dest_dir", destDir,
			"err", cloneErr,
			"git_output", output,
		)
		h.errorToast(w, app.FriendlyMessage(cloneErr))
		w.WriteHeader(http.StatusInternalServerError)
		h.render(w, "repos/clone-result.html", map[string]any{
			"OK":      false,
			"Message": "Falha ao clonar repositório.",
			"SSHURL":  sshURL,
			"Output":  output,
		})
		return
	}

	h.successToastOnly(w, "Repositório clonado com sucesso!")
	h.render(w, "repos/clone-result.html", map[string]any{
		"OK":      true,
		"Message": "Clone concluído.",
		"SSHURL":  sshURL,
		"Output":  output,
	})
}

func (h *Handler) repoAccounts() ([]reposAccountView, error) {
	state, err := h.app.Storage.LoadState()
	if err != nil {
		return nil, err
	}
	out := make([]reposAccountView, 0, len(state.Accounts))
	for _, a := range state.Accounts {
		_, err := os.Stat(h.app.Paths.PrivateKey(a.HostAlias))
		out = append(out, reposAccountView{
			ID:        a.ID,
			Name:      a.Name,
			HostAlias: a.HostAlias,
			HostName:  a.HostName,
			HasKey:    err == nil,
		})
	}
	return out, nil
}

func findAccountByID(accounts []storage.Account, id string) (storage.Account, bool) {
	for _, a := range accounts {
		if a.ID == id {
			return a, true
		}
	}
	return storage.Account{}, false
}
