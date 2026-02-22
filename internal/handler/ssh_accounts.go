package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"regexp"
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
	KeyTypeLabel string
}

type accountFormData struct {
	Account          storage.Account
	Errors           map[string]string
	SubmitURL        string
	IsEdit           bool
	ShowAliasWarning bool
}

type simpleKeyFormData struct {
	Account   storage.Account
	Errors    map[string]string
	SubmitURL string
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
		kt := a.EffectiveKeyType()
		privPath := h.app.Paths.PrivateKeyForType(a.HostAlias, kt)
		pubPath := h.app.Paths.PublicKeyForType(a.HostAlias, kt)
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
			SSHPreview:   buildSSHPreview(a, privPath),
			KeyErrorHint: keyErrorHint,
			KeyTypeLabel: keyTypeLabel(kt),
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

// ── Modo simples (Nova Chave) ─────────────────────────────────────

func (h *Handler) NewSimpleKeyDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Nova Chave SSH", "ssh/quick-key-drawer.html", simpleKeyFormData{
		Account:   storage.Account{KeyType: "ed25519"},
		Errors:    map[string]string{},
		SubmitURL: "/tools/ssh/keys",
	})
}

func (h *Handler) CreateSimpleKey(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	keyType := r.FormValue("keyType")
	if keyType != "rsa4096" {
		keyType = "ed25519"
	}

	alias := sanitizeAlias(name)
	a := storage.Account{
		ID:          newID(),
		Name:        name,
		HostAlias:   alias,
		KeyType:     keyType,
		IsSimpleKey: true,
		Provider:    "other",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	// Permite alias customizado do formulário
	if custom := strings.TrimSpace(r.FormValue("hostAlias")); custom != "" {
		a.HostAlias = custom
	}
	a.IdentityFile = h.app.Paths.PrivateKeyForType(a.HostAlias, keyType)

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	if errs := storage.ValidateSimple(a, state.Accounts); len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderDrawer(w, "Nova Chave SSH", "ssh/quick-key-drawer.html", simpleKeyFormData{
			Account:   a,
			Errors:    mapValidation(errs),
			SubmitURL: "/tools/ssh/keys",
		})
		return
	}

	state.Accounts = append(state.Accounts, a)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	if err := ssh.GenerateKey(a.HostAlias, a.Name, keyType, h.app.Paths); err != nil && err != ssh.ErrKeyExists {
		h.app.Logger.Warn("falha ao gerar chave simples", "alias", a.HostAlias, "err", err)
	}

	h.successToast(w, "Chave criada com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListAccounts(w, r)
}

// ── Modo completo (Nova Conta) ─────────────────────────────────────

func (h *Handler) NewAccountDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Nova Conta SSH", "ssh/account-drawer.html", accountFormData{
		Account:   storage.Account{Provider: "github", KeyType: "ed25519"},
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

	_, keyErr := os.Stat(h.app.Paths.PrivateKeyForType(a.HostAlias, a.EffectiveKeyType()))
	showAliasWarning := keyErr == nil

	h.renderDrawer(w, "Configurar Conta SSH", "ssh/account-drawer.html", accountFormData{
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
	a.IdentityFile = h.app.Paths.PrivateKeyForType(a.HostAlias, a.EffectiveKeyType())

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	if errs := storage.Validate(a, state.Accounts); len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderDrawer(w, "Nova Conta SSH", "ssh/account-drawer.html", accountFormData{
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

	kt := a.EffectiveKeyType()
	if err := ssh.GenerateKey(a.HostAlias, a.GitUserEmail, kt, h.app.Paths); err != nil && err != ssh.ErrKeyExists {
		h.app.Logger.Warn("falha ao gerar chave automaticamente", "alias", a.HostAlias, "err", err)
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
	a.KeyType = old.KeyType // preserva o tipo de chave original
	a.CreatedAt = old.CreatedAt
	a.UpdatedAt = time.Now()
	a.IdentityFile = h.app.Paths.PrivateKeyForType(a.HostAlias, a.EffectiveKeyType())
	// Se preencheu todos os campos, deixa de ser chave simples
	if a.Provider != "" && a.HostName != "" && a.GitUserName != "" && a.GitUserEmail != "" {
		a.IsSimpleKey = false
	} else {
		a.IsSimpleKey = old.IsSimpleKey
	}

	if errs := storage.Validate(a, state.Accounts); len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		showAliasWarning := old.HostAlias != a.HostAlias
		h.renderDrawer(w, "Configurar Conta SSH", "ssh/account-drawer.html", accountFormData{
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
	kt := target.EffectiveKeyType()
	if _, err := os.Stat(h.app.Paths.PrivateKeyForType(target.HostAlias, kt)); err == nil {
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

	kt := a.EffectiveKeyType()
	comment := a.GitUserEmail
	if comment == "" {
		comment = a.Name
	}
	if err := ssh.ForceGenerateKey(a.HostAlias, comment, kt, h.app.Paths); err != nil {
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

// ── Helpers internos ──────────────────────────────────────────────

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
	kt := r.FormValue("keyType")
	if kt != "rsa4096" {
		kt = "ed25519"
	}
	return storage.Account{
		Name:         r.FormValue("name"),
		Provider:     r.FormValue("provider"),
		HostName:     r.FormValue("hostName"),
		HostAlias:    r.FormValue("hostAlias"),
		GitUserName:  r.FormValue("gitUserName"),
		GitUserEmail: r.FormValue("gitUserEmail"),
		KeyType:      kt,
	}
}

func newID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func buildSSHPreview(a storage.Account, identityPath string) string {
	if a.IsSimpleKey || a.HostName == "" {
		return "# Chave simples — configure SSH alias para usar com git clone"
	}
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

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeAlias transforma um nome qualquer em alias válido (lowercase, sem espaços).
func sanitizeAlias(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "key"
	}
	return s
}

func keyTypeLabel(kt string) string {
	switch kt {
	case "rsa4096":
		return "RSA 4096"
	default:
		return "Ed25519"
	}
}
