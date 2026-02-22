package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
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

type repoView struct {
	storage.Repository
	AccountName string
	Branch      string
	IsClean     bool
	StatusShort string
	Accessible  bool
}

func (h *Handler) Repositories(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	accountMap := make(map[string]string, len(state.Accounts))
	for _, a := range state.Accounts {
		accountMap[a.ID] = a.Name
	}

	views := make([]repoView, len(state.Repositories))
	for i, repo := range state.Repositories {
		views[i] = repoView{
			Repository:  repo,
			AccountName: accountMap[repo.AccountID],
		}
	}

	// Busca status git em paralelo com timeout
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := range views {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			branch, statusShort, isClean, accessible := fetchRepoStatus(views[idx].LocalPath)
			mu.Lock()
			views[idx].Branch = branch
			views[idx].StatusShort = statusShort
			views[idx].IsClean = isClean
			views[idx].Accessible = accessible
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	accounts, err := h.repoAccounts()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	payload := map[string]any{
		"Repos":    views,
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

// RepoStatus retorna o status atualizado de um repositório individual (partial HTML).
func (h *Handler) RepoStatus(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	var found *storage.Repository
	for i := range state.Repositories {
		if state.Repositories[i].ID == id {
			found = &state.Repositories[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Repositório não encontrado", http.StatusNotFound)
		return
	}

	branch, statusShort, isClean, accessible := fetchRepoStatus(found.LocalPath)
	h.render(w, "repos/status-badge.html", map[string]any{
		"Branch":      branch,
		"StatusShort": statusShort,
		"IsClean":     isClean,
		"Accessible":  accessible,
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

// StartCloneJob inicia o clone de forma assíncrona e retorna imediatamente
// com um spinner. O cliente faz polling em /tools/repos/jobs/{id}.
func (h *Handler) StartCloneJob(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	accountID := strings.TrimSpace(r.FormValue("accountID"))
	rawURL := strings.TrimSpace(r.FormValue("repoURL"))
	destDir := strings.TrimSpace(r.FormValue("destDir"))

	if accountID == "" || rawURL == "" || destDir == "" {
		h.render(w, "repos/clone-progress.html", map[string]any{
			"Done": true, "OK": false,
			"Error": "Preencha conta, URL e diretório de destino.",
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

	identityPath := h.app.Paths.PrivateKeyForType(account.HostAlias, account.EffectiveKeyType())
	if _, err := os.Stat(identityPath); err != nil {
		h.render(w, "repos/clone-progress.html", map[string]any{
			"Done": true, "OK": false,
			"Error": "A conta selecionada não possui chave privada. Gere a chave primeiro.",
		})
		return
	}

	sshURL, err := h.app.GitService.BuildSSHURL(rawURL, account.HostAlias)
	if err != nil {
		h.render(w, "repos/clone-progress.html", map[string]any{
			"Done": true, "OK": false,
			"Error": "URL de repositório inválida: " + err.Error(),
		})
		return
	}

	job := &CloneJob{ID: newID()}
	h.cloneMu.Lock()
	h.cloneJobs[job.ID] = job
	h.cloneMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		output, cloneErr := h.app.GitService.CloneRepo(ctx, sshURL, destDir, identityPath)
		if cloneErr != nil {
			h.app.Logger.Error("falha no clone", "alias", account.HostAlias, "err", cloneErr)
			h.cloneMu.Lock()
			job.Done, job.OK = true, false
			job.Output = output
			job.Error = app.FriendlyMessage(cloneErr)
			h.cloneMu.Unlock()
			return
		}

		repoName := repoNameFromURL(rawURL)
		repo := storage.Repository{
			ID:        newID(),
			AccountID: accountID,
			Name:      repoName,
			URL:       rawURL,
			LocalPath: destDir,
			ClonedAt:  time.Now(),
		}
		if st, loadErr := h.app.Storage.LoadState(); loadErr == nil {
			st.Repositories = append(st.Repositories, repo)
			if saveErr := h.app.Storage.SaveState(st); saveErr != nil {
				h.app.Logger.Warn("falha ao salvar repo no state", "err", saveErr)
			}
		}

		h.cloneMu.Lock()
		job.Done, job.OK = true, true
		h.cloneMu.Unlock()
	}()

	h.render(w, "repos/clone-progress.html", map[string]any{
		"ID": job.ID, "Done": false,
	})
}

// CloneJobStatus retorna o estado atual do job (polling HTMX).
// Retorna HTTP 286 quando concluído, parando o polling automaticamente.
func (h *Handler) CloneJobStatus(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")

	h.cloneMu.Lock()
	job, ok := h.cloneJobs[id]
	var done, jobOK bool
	var errMsg, output string
	if ok {
		done, jobOK = job.Done, job.OK
		errMsg, output = job.Error, job.Output
	}
	h.cloneMu.Unlock()

	if !ok {
		h.render(w, "repos/clone-progress.html", map[string]any{
			"Done": true, "OK": false, "ID": id,
			"Error": "Job de clone não encontrado.",
		})
		return
	}

	if done {
		if jobOK {
			w.Header().Set("HX-Trigger", `{"showToast":{"msg":"Repositório clonado com sucesso!","type":"success"},"closeDrawer":true,"refreshRepos":true}`)
		} else {
			h.errorToast(w, errMsg)
		}
		w.WriteHeader(286) // para o polling HTMX
	}

	h.render(w, "repos/clone-progress.html", map[string]any{
		"ID":     id,
		"Done":   done,
		"OK":     jobOK,
		"Error":  errMsg,
		"Output": output,
	})
}

func (h *Handler) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i, repo := range state.Repositories {
		if repo.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		h.operationError(w, "Repositório não encontrado", http.StatusNotFound)
		return
	}
	state.Repositories = append(state.Repositories[:idx], state.Repositories[idx+1:]...)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.repoSuccessToast(w, "Repositório removido com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.Repositories(w, r)
}

// ── Helpers internos ──────────────────────────────────────────────

func fetchRepoStatus(localPath string) (branch, statusShort string, isClean, accessible bool) {
	if _, err := os.Stat(localPath); err != nil {
		return "?", "caminho não encontrado", false, false
	}
	accessible = true

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// Branch atual
	branchOut, err := exec.CommandContext(ctx, "git", "-C", localPath, "branch", "--show-current").Output()
	if err == nil {
		branch = strings.TrimSpace(string(branchOut))
	}
	if branch == "" {
		// Pode estar em modo detached HEAD
		hashOut, _ := exec.CommandContext(ctx, "git", "-C", localPath, "rev-parse", "--short", "HEAD").Output()
		branch = "HEAD:" + strings.TrimSpace(string(hashOut))
	}

	// Status curto
	statusOut, err := exec.CommandContext(ctx, "git", "-C", localPath, "status", "--short").Output()
	if err == nil {
		statusStr := strings.TrimSpace(string(statusOut))
		if statusStr == "" {
			isClean = true
			statusShort = "limpo"
		} else {
			n := len(strings.Split(statusStr, "\n"))
			statusShort = fmt.Sprintf("%d alteração(ões)", n)
		}
	} else {
		statusShort = "erro ao ler"
	}
	return
}

func repoNameFromURL(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, ".git")
	parts := strings.Split(rawURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return rawURL
}

func (h *Handler) repoAccounts() ([]reposAccountView, error) {
	state, err := h.app.Storage.LoadState()
	if err != nil {
		return nil, err
	}
	out := make([]reposAccountView, 0, len(state.Accounts))
	for _, a := range state.Accounts {
		_, err := os.Stat(h.app.Paths.PrivateKeyForType(a.HostAlias, a.EffectiveKeyType()))
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
