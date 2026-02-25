package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/app"
	igit "github.com/seuusuario/factorydev/internal/git"
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
	AccountName  string
	Branch       string
	IsClean      bool
	StatusShort  string
	Accessible   bool
	LastCommit   *igit.CommitEntry
	IdentityName string
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
		w.WriteHeader(286)
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

// ── Scan Repos ────────────────────────────────────────────────────

type excludeOption struct {
	Name    string
	Default bool
}

func (h *Handler) ScanReposDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	opts := make([]excludeOption, 0, len(igit.DefaultScanExcludes))
	for _, d := range igit.DefaultScanExcludes {
		opts = append(opts, excludeOption{Name: d, Default: true})
	}
	h.renderDrawer(w, "Escanear Repositórios", "repos/scan-drawer.html", map[string]any{
		"DefaultPath":    h.app.Paths.Home,
		"ExcludeOptions": opts,
	})
}

func (h *Handler) ValidateScanPath(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	root := strings.TrimSpace(r.FormValue("scanPath"))
	if root == "" {
		h.operationError(w, "Caminho é obrigatório", http.StatusBadRequest)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	managedPaths := make(map[string]bool, len(state.Repositories))
	for _, repo := range state.Repositories {
		managedPaths[repo.LocalPath] = true
	}

	excludeDirs := r.Form["excludeDirs"]
	if len(excludeDirs) == 0 {
		excludeDirs = igit.DefaultScanExcludes
	}

	svc := igit.NewService()
	found, err := svc.ScanForRepos(root, 4, excludeDirs)
	if err != nil {
		h.operationError(w, "Erro ao escanear: "+err.Error(), http.StatusBadRequest)
		return
	}

	type scanCandidate struct {
		Path         string
		Name         string
		AlreadyInUse bool
	}
	var candidates []scanCandidate
	for _, p := range found {
		candidates = append(candidates, scanCandidate{
			Path:         p,
			Name:         filepath.Base(p),
			AlreadyInUse: managedPaths[p],
		})
	}

	h.render(w, "repos/scan-preview.html", map[string]any{
		"Candidates": candidates,
		"Total":      len(candidates),
	})
}

func (h *Handler) ImportScannedRepos(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	paths := r.Form["repoPaths"]
	if len(paths) == 0 {
		h.successToast(w, "Nenhum repositório selecionado.")
		h.Repositories(w, r)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	managedPaths := make(map[string]bool)
	for _, repo := range state.Repositories {
		managedPaths[repo.LocalPath] = true
	}

	imported := 0
	for _, p := range paths {
		if managedPaths[p] {
			continue
		}
		state.Repositories = append(state.Repositories, storage.Repository{
			ID:        newID(),
			Name:      filepath.Base(p),
			LocalPath: p,
			ClonedAt:  time.Now(),
		})
		imported++
	}

	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	alreadyManaged := len(paths) - imported
	msg := fmt.Sprintf("%d repositório(s) importado(s)", imported)
	if alreadyManaged > 0 {
		msg += fmt.Sprintf(", %d já gerenciado(s)", alreadyManaged)
	}
	h.successToast(w, msg)
	h.Repositories(w, r)
}

// ── Pull Job ──────────────────────────────────────────────────────

func (h *Handler) StartPullJob(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	var repo *storage.Repository
	for i := range state.Repositories {
		if state.Repositories[i].ID == id {
			repo = &state.Repositories[i]
			break
		}
	}
	if repo == nil {
		h.operationError(w, "Repositório não encontrado", http.StatusNotFound)
		return
	}

	identityFile := ""
	for _, acc := range state.Accounts {
		if acc.ID == repo.AccountID {
			for _, k := range state.Keys {
				if k.ID == acc.KeyID {
					identityFile = k.PrivateKeyPath
					break
				}
			}
			if identityFile == "" {
				identityFile = acc.IdentityFile
			}
			break
		}
	}

	job := &GitOpJob{ID: newID()}
	h.pullMu.Lock()
	h.pullJobs[job.ID] = job
	h.pullMu.Unlock()

	localPath := repo.LocalPath
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		svc := igit.NewService()
		output, pullErr := svc.Pull(ctx, localPath, identityFile)
		h.pullMu.Lock()
		job.Output = output
		if pullErr != nil {
			job.Done, job.OK = true, false
			job.Error = app.FriendlyMessage(pullErr)
		} else {
			job.Done, job.OK = true, true
		}
		h.pullMu.Unlock()
	}()

	h.render(w, "repos/pull-progress.html", map[string]any{
		"ID": job.ID, "Done": false, "RepoID": id,
	})
}

func (h *Handler) PullJobStatus(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	jobID := chi.URLParam(r, "jobId")
	repoID := r.URL.Query().Get("repoId")

	h.pullMu.Lock()
	job, ok := h.pullJobs[jobID]
	var done, jobOK bool
	var errMsg, output string
	if ok {
		done, jobOK = job.Done, job.OK
		errMsg, output = job.Error, job.Output
	}
	h.pullMu.Unlock()

	if !ok {
		h.render(w, "repos/pull-progress.html", map[string]any{
			"Done": true, "OK": false, "ID": jobID, "RepoID": repoID,
			"Error": "Job não encontrado.",
		})
		return
	}
	if done {
		if jobOK {
			w.Header().Set("HX-Trigger", `{"showToast":{"msg":"Pull concluído!","type":"success"}}`)
		} else {
			h.errorToast(w, errMsg)
		}
		w.WriteHeader(286)
	}
	h.render(w, "repos/pull-progress.html", map[string]any{
		"ID":     jobID,
		"RepoID": repoID,
		"Done":   done,
		"OK":     jobOK,
		"Error":  errMsg,
		"Output": output,
	})
}

// ── New Branch ────────────────────────────────────────────────────

func (h *Handler) NewBranchHandler(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	branch := strings.TrimSpace(r.FormValue("branch"))
	if branch == "" {
		h.operationError(w, "Nome do branch é obrigatório", http.StatusBadRequest)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var localPath string
	for _, repo := range state.Repositories {
		if repo.ID == id {
			localPath = repo.LocalPath
			break
		}
	}
	if localPath == "" {
		h.operationError(w, "Repositório não encontrado", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	svc := igit.NewService()
	if err := svc.NewBranch(ctx, localPath, branch); err != nil {
		h.errorToast(w, "Erro ao criar branch: "+err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToastOnly(w, "Branch '"+branch+"' criado e ativo!")
}

// ── Repo Tabs ─────────────────────────────────────────────────────

func (h *Handler) GetRepoTab(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	tab := chi.URLParam(r, "tab")

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	var repo *storage.Repository
	for i := range state.Repositories {
		if state.Repositories[i].ID == id {
			repo = &state.Repositories[i]
			break
		}
	}
	if repo == nil {
		h.operationError(w, "Repositório não encontrado", http.StatusNotFound)
		return
	}

	svc := igit.NewService()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch tab {
	case "overview":
		commits, _ := svc.GetLog(ctx, repo.LocalPath, 1, 0)
		var lastCommit *igit.CommitEntry
		if len(commits) > 0 {
			lastCommit = &commits[0]
		}
		h.render(w, "repos/tab-overview.html", map[string]any{
			"Repo":       repo,
			"LastCommit": lastCommit,
		})
	case "config":
		cfg, _ := svc.GetRepoGitConfig(repo.LocalPath)
		h.render(w, "repos/tab-config.html", map[string]any{
			"Repo":       repo,
			"GitConfig":  cfg,
			"Identities": state.Identities,
		})
	case "commits":
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		commits, _ := svc.GetLog(ctx, repo.LocalPath, 20, offset)
		h.render(w, "repos/tab-commits.html", map[string]any{
			"Repo":    repo,
			"Commits": commits,
			"Offset":  offset + 20,
			"HasMore": len(commits) == 20,
		})
	case "branches":
		branches, err := svc.ListBranches(ctx, repo.LocalPath)
		h.render(w, "repos/tab-branches.html", map[string]any{
			"Repo":     repo,
			"Branches": branches,
			"Err":      err,
		})
	default:
		h.operationError(w, "Tab desconhecida", http.StatusBadRequest)
	}
}

// CheckoutBranch faz checkout de um branch existente no repositório.
func (h *Handler) CheckoutBranch(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	branch := strings.TrimSpace(r.FormValue("branch"))
	if branch == "" {
		h.errorToast(w, "Nome do branch é obrigatório")
		return
	}
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var repo *storage.Repository
	for i := range state.Repositories {
		if state.Repositories[i].ID == id {
			repo = &state.Repositories[i]
			break
		}
	}
	if repo == nil {
		h.errorToast(w, "Repositório não encontrado")
		return
	}
	svc := igit.NewService()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := svc.CheckoutBranch(ctx, repo.LocalPath, branch); err != nil {
		h.errorToast(w, "Erro ao trocar branch: "+err.Error())
		return
	}
	h.repoSuccessToast(w, "Branch alterado para "+branch)
	branches, _ := svc.ListBranches(ctx, repo.LocalPath)
	h.render(w, "repos/tab-branches.html", map[string]any{
		"Repo":     repo,
		"Branches": branches,
	})
}

// ── Pull All Job ──────────────────────────────────────────────────

func (h *Handler) StartPullAllJob(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	force := r.FormValue("force") == "1" || r.FormValue("force") == "true"

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	if len(state.Repositories) == 0 {
		h.successToastOnly(w, "Nenhum repositório para atualizar.")
		return
	}

	// Mapa accountID -> arquivo de identidade
	keyMap := make(map[string]string)
	for _, acc := range state.Accounts {
		for _, k := range state.Keys {
			if k.ID == acc.KeyID {
				keyMap[acc.ID] = k.PrivateKeyPath
				break
			}
		}
		if _, ok := keyMap[acc.ID]; !ok && acc.IdentityFile != "" {
			keyMap[acc.ID] = acc.IdentityFile
		}
	}

	job := &PullAllJob{
		ID:      newID(),
		Force:   force,
		Total:   len(state.Repositories),
		Results: make([]PullAllResult, len(state.Repositories)),
	}
	for i, repo := range state.Repositories {
		job.Results[i] = PullAllResult{RepoName: repo.Name}
	}

	h.pullAllMu.Lock()
	h.pullAllJobs[job.ID] = job
	h.pullAllMu.Unlock()

	for i, repo := range state.Repositories {
		go func(idx int, r storage.Repository) {
			identityFile := keyMap[r.AccountID]
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			svc := igit.NewService()
			var output string
			var pullErr error
			if force {
				output, pullErr = svc.PullForce(ctx, r.LocalPath, identityFile)
			} else {
				output, pullErr = svc.Pull(ctx, r.LocalPath, identityFile)
			}

			h.pullAllMu.Lock()
			job.Results[idx].Done = true
			job.Results[idx].Output = output
			if pullErr != nil {
				job.Results[idx].OK = false
				job.Results[idx].Error = app.FriendlyMessage(pullErr)
			} else {
				job.Results[idx].OK = true
			}
			allDone := true
			for _, res := range job.Results {
				if !res.Done {
					allDone = false
					break
				}
			}
			if allDone {
				job.Done = true
			}
			h.pullAllMu.Unlock()
		}(i, repo)
	}

	h.render(w, "repos/pull-all-progress.html", map[string]any{
		"ID":      job.ID,
		"Done":    false,
		"Force":   force,
		"Results": job.Results,
	})
}

func (h *Handler) PullAllJobStatus(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")

	h.pullAllMu.Lock()
	job, ok := h.pullAllJobs[id]
	var done, force bool
	var results []PullAllResult
	if ok {
		done = job.Done
		force = job.Force
		results = make([]PullAllResult, len(job.Results))
		copy(results, job.Results)
	}
	h.pullAllMu.Unlock()

	if !ok {
		h.operationError(w, "Job não encontrado", http.StatusNotFound)
		return
	}

	if done {
		allOK := true
		for _, res := range results {
			if !res.OK {
				allOK = false
				break
			}
		}
		if allOK {
			w.Header().Set("HX-Trigger", `{"showToast":{"msg":"Pull concluído em todos os repositórios!","type":"success"}}`)
		}
		w.WriteHeader(286)
	}

	h.render(w, "repos/pull-all-progress.html", map[string]any{
		"ID":      id,
		"Done":    done,
		"Force":   force,
		"Results": results,
	})
}

func (h *Handler) SetRepoGitConfigHandler(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var localPath string
	for _, repo := range state.Repositories {
		if repo.ID == id {
			localPath = repo.LocalPath
			break
		}
	}
	if localPath == "" {
		h.operationError(w, "Repositório não encontrado", http.StatusNotFound)
		return
	}

	svc := igit.NewService()
	kvs := map[string]string{
		"user.name":  strings.TrimSpace(r.FormValue("userName")),
		"user.email": strings.TrimSpace(r.FormValue("userEmail")),
	}
	if v := strings.TrimSpace(r.FormValue("gpgFormat")); v != "" {
		kvs["gpg.format"] = v
	}
	if v := strings.TrimSpace(r.FormValue("signingKey")); v != "" {
		kvs["user.signingkey"] = v
	}
	if v := strings.TrimSpace(r.FormValue("commitGpgSign")); v == "true" || v == "1" {
		kvs["commit.gpgsign"] = "true"
	}

	for k, v := range kvs {
		if v == "" {
			continue
		}
		if err := svc.SetRepoGitConfig(localPath, k, v); err != nil {
			h.errorToast(w, "Erro ao salvar "+k+": "+err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
	}
	h.successToastOnly(w, "Configuração git local salva!")
}

func (h *Handler) OpenRepoTerminal(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var localPath string
	for _, repo := range state.Repositories {
		if repo.ID == id {
			localPath = repo.LocalPath
			break
		}
	}
	if localPath == "" {
		h.operationError(w, "Repositório não encontrado", http.StatusNotFound)
		return
	}

	if err := igit.OpenTerminalAt(localPath); err != nil {
		h.errorToast(w, err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToastOnly(w, "Terminal aberto!")
}

// ── Helpers internos ──────────────────────────────────────────────

func fetchRepoStatus(localPath string) (branch, statusShort string, isClean, accessible bool) {
	if _, err := os.Stat(localPath); err != nil {
		return "?", "caminho não encontrado", false, false
	}
	accessible = true

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	branchOut, err := exec.CommandContext(ctx, "git", "-C", localPath, "branch", "--show-current").Output()
	if err == nil {
		branch = strings.TrimSpace(string(branchOut))
	}
	if branch == "" {
		hashOut, _ := exec.CommandContext(ctx, "git", "-C", localPath, "rev-parse", "--short", "HEAD").Output()
		branch = "HEAD:" + strings.TrimSpace(string(hashOut))
	}

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
