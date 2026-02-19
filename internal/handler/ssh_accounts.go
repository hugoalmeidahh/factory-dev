package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/ssh"
	"github.com/seuusuario/factorydev/internal/storage"
)

type accountView struct {
	storage.Account
	HasKey       bool
	PrivateKey   string
	PublicKey    string
	SSHPreview   string
	KeyErrorHint string
}

type accountFormData struct {
	Account          storage.Account
	Errors           map[string]string
	SubmitURL        string
	IsEdit           bool
	ShowAliasWarning bool
}

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	accounts := make([]accountView, 0, len(state.Accounts))
	for _, a := range state.Accounts {
		privPath := h.app.Paths.PrivateKey(a.HostAlias)
		pubPath := h.app.Paths.PublicKey(a.HostAlias)
		_, err := os.Stat(privPath)
		hasKey := err == nil
		var privText, pubText, keyErrorHint string
		if hasKey {
			privBytes, privErr := os.ReadFile(privPath)
			pubBytes, pubErr := os.ReadFile(pubPath)
			if privErr == nil {
				privText = string(privBytes)
			}
			if pubErr == nil {
				pubText = string(pubBytes)
			}
			if privErr != nil || pubErr != nil {
				keyErrorHint = "Não foi possível ler uma das chaves no disco."
			}
		}
		accounts = append(accounts, accountView{
			Account:      a,
			HasKey:       hasKey,
			PrivateKey:   strings.TrimSpace(privText),
			PublicKey:    strings.TrimSpace(pubText),
			SSHPreview:   buildSSHPreview(a, h.app.Paths.PrivateKey(a.HostAlias)),
			KeyErrorHint: keyErrorHint,
		})
	}

	data := PageData{
		Title:      "FactoryDev",
		ActiveTool: "ssh",
		ContentTpl: "ssh/accounts-list.html",
		Data: map[string]any{
			"Accounts": accounts,
		},
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "ssh/accounts-list.html", data.Data)
		return
	}
	h.render(w, "ssh/accounts-list.html", data)
}

func (h *Handler) NewAccountDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Nova Conta", "ssh/account-drawer.html", accountFormData{
		Account:   storage.Account{Provider: "github"},
		Errors:    map[string]string{},
		SubmitURL: "/tools/ssh/accounts",
	})
}

func (h *Handler) EditAccountDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	idx := findAccountIndex(state.Accounts, id)
	if idx < 0 {
		h.operationError(w, "Conta não encontrada", http.StatusNotFound)
		return
	}
	a := state.Accounts[idx]

	_, keyErr := os.Stat(h.app.Paths.PrivateKey(a.HostAlias))
	showAliasWarning := keyErr == nil

	h.renderDrawer(w, "Editar Conta", "ssh/account-drawer.html", accountFormData{
		Account:          a,
		Errors:           map[string]string{},
		SubmitURL:        "/tools/ssh/accounts/" + id,
		IsEdit:           true,
		ShowAliasWarning: showAliasWarning,
	})
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	a := accountFromRequest(r)
	a.ID = newID()
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	a.IdentityFile = h.app.Paths.PrivateKey(a.HostAlias)

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	if errs := storage.Validate(a, state.Accounts); len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderDrawer(w, "Nova Conta", "ssh/account-drawer.html", accountFormData{
			Account:   a,
			Errors:    mapValidation(errs),
			SubmitURL: "/tools/ssh/accounts",
		})
		return
	}

	state.Accounts = append(state.Accounts, a)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	h.successToast(w, "Conta criada com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListAccounts(w, r)
}

func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
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
	idx := findAccountIndex(state.Accounts, id)
	if idx < 0 {
		h.operationError(w, "Conta não encontrada", http.StatusNotFound)
		return
	}

	old := state.Accounts[idx]
	a := accountFromRequest(r)
	a.ID = old.ID
	a.CreatedAt = old.CreatedAt
	a.UpdatedAt = time.Now()
	a.IdentityFile = h.app.Paths.PrivateKey(a.HostAlias)

	if errs := storage.Validate(a, state.Accounts); len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		showAliasWarning := false
		if old.HostAlias != a.HostAlias {
			if _, err := os.Stat(h.app.Paths.PrivateKey(old.HostAlias)); err == nil {
				showAliasWarning = true
			}
		}
		h.renderDrawer(w, "Editar Conta", "ssh/account-drawer.html", accountFormData{
			Account:          a,
			Errors:           mapValidation(errs),
			SubmitURL:        "/tools/ssh/accounts/" + id,
			IsEdit:           true,
			ShowAliasWarning: showAliasWarning,
		})
		return
	}

	state.Accounts[idx] = a
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	h.successToast(w, "Conta atualizada com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListAccounts(w, r)
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	idx := findAccountIndex(state.Accounts, id)
	if idx < 0 {
		h.operationError(w, "Conta não encontrada", http.StatusNotFound)
		return
	}
	target := state.Accounts[idx]
	if _, err := os.Stat(h.app.Paths.PrivateKey(target.HostAlias)); err == nil {
		h.app.Logger.Warn("conta removida mantendo chave no disco", "alias", target.HostAlias)
	}

	state.Accounts = append(state.Accounts[:idx], state.Accounts[idx+1:]...)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	h.successToast(w, "Conta removida com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListAccounts(w, r)
}

func (h *Handler) GenerateKey(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	a, ok := h.accountByID(w, r)
	if !ok {
		return
	}

	if err := ssh.GenerateKey(a.HostAlias, a.GitUserEmail, h.app.Paths); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusBadRequest)
		return
	}

	h.successToast(w, "Chave gerada com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListAccounts(w, r)
}

func (h *Handler) ApplySSHConfig(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	a, ok := h.accountByID(w, r)
	if !ok {
		return
	}

	if err := ssh.ApplyAccount(a, h.app.Paths); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	h.successToast(w, "SSH config aplicado com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListAccounts(w, r)
}

func (h *Handler) PreviewApplySSHConfig(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	a, ok := h.accountByID(w, r)
	if !ok {
		return
	}
	lines, err := ssh.PreviewApply(a, h.app.Paths)
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.render(w, "ssh/diff-preview.html", map[string]any{"Lines": lines})
}

func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	a, ok := h.accountByID(w, r)
	if !ok {
		return
	}

	output, err := ssh.TestConnection(a.HostAlias)
	if err != nil {
		h.app.Logger.Warn("falha no teste ssh", "alias", a.HostAlias, "err", err)
	}
	h.render(w, "ssh/test-result.html", map[string]any{
		"Output": output,
		"Error":  app.FriendlyMessage(err),
		"OK":     err == nil,
	})
}

func (h *Handler) accountByID(w http.ResponseWriter, r *http.Request) (storage.Account, bool) {
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return storage.Account{}, false
	}
	idx := findAccountIndex(state.Accounts, id)
	if idx < 0 {
		h.operationError(w, "Conta não encontrada", http.StatusNotFound)
		return storage.Account{}, false
	}
	return state.Accounts[idx], true
}

func findAccountIndex(accounts []storage.Account, id string) int {
	for i := range accounts {
		if accounts[i].ID == id {
			return i
		}
	}
	return -1
}

func accountFromRequest(r *http.Request) storage.Account {
	return storage.Account{
		Name:         r.FormValue("name"),
		Provider:     r.FormValue("provider"),
		HostName:     r.FormValue("hostName"),
		HostAlias:    r.FormValue("hostAlias"),
		GitUserName:  r.FormValue("gitUserName"),
		GitUserEmail: r.FormValue("gitUserEmail"),
	}
}

func newID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func buildSSHPreview(a storage.Account, identityPath string) string {
	return strings.Join([]string{
		"# BEGIN FDEV " + a.HostAlias,
		"Host " + a.HostAlias,
		"  HostName " + a.HostName,
		"  User git",
		"  IdentityFile " + identityPath,
		"  IdentitiesOnly yes",
		"# END FDEV " + a.HostAlias,
	}, "\n")
}
