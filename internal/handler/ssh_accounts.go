package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
	KeyName      string // nome da chave vinculada (se houver)
}

type accountFormData struct {
	Account          storage.Account
	Keys             []storage.Key // chaves disponíveis para seleção
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

	// Mapa rápido: keyID → Key
	keyMap := make(map[string]storage.Key, len(state.Keys))
	for _, k := range state.Keys {
		keyMap[k.ID] = k
	}

	accounts := make([]accountView, 0, len(state.Accounts))
	for _, a := range state.Accounts {
		var privPath, pubPath, keyTyp, keyName string

		if a.KeyID != "" {
			if k, ok := keyMap[a.KeyID]; ok {
				privPath = k.PrivateKeyPath
				pubPath = k.PublicKeyPath
				keyTyp = k.Type
				if k.Bits > 0 {
					keyTyp = fmt.Sprintf("%s-%d", k.Type, k.Bits)
				}
				keyName = k.Name
			}
		} else if a.IdentityFile != "" {
			// Legacy: usa IdentityFile diretamente
			kt := a.EffectiveKeyType()
			privPath = h.app.Paths.PrivateKeyForType(a.HostAlias, kt)
			pubPath = h.app.Paths.PublicKeyForType(a.HostAlias, kt)
			keyTyp = kt
		}

		var hasKey bool
		var privText, pubText, keyErrorHint string
		if privPath != "" {
			_, err := os.Stat(privPath)
			hasKey = err == nil
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
		}

		accounts = append(accounts, accountView{
			Account:      a,
			HasKey:       hasKey,
			PrivateKey:   strings.TrimSpace(privText),
			PublicKey:    strings.TrimSpace(pubText),
			SSHPreview:   buildSSHPreview(a, privPath),
			KeyErrorHint: keyErrorHint,
			KeyTypeLabel: keyTypeLabel(keyTyp),
			KeyName:      keyName,
		})
	}

	data := map[string]any{"Accounts": accounts}
	pageData := PageData{
		Title:      "FactoryDev",
		ActiveTool: "ssh",
		ContentTpl: "ssh/accounts-list.html",
		Data:       data,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "ssh/accounts-list.html", data)
		return
	}
	h.render(w, "ssh/accounts-list.html", pageData)
}

func (h *Handler) NewAccountDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.renderDrawer(w, "Nova Conta SSH", "ssh/account-drawer.html", accountFormData{
		Account:   storage.Account{Provider: "github"},
		Keys:      state.Keys,
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

	showAliasWarning := a.KeyID != "" || a.IdentityFile != ""

	h.renderDrawer(w, "Configurar Conta SSH", "ssh/account-drawer.html", accountFormData{
		Account:          a,
		Keys:             state.Keys,
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

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	if errs := storage.Validate(a, state.Accounts); len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderDrawer(w, "Nova Conta SSH", "ssh/account-drawer.html", accountFormData{
			Account:   a,
			Keys:      state.Keys,
			Errors:    mapValidation(errs),
			SubmitURL: "/tools/ssh/accounts",
		})
		return
	}

	// Resolver chave (selecionar existente ou criar inline)
	if err := h.resolveAccountKey(r, &a, state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	// Atualizar IdentityFile para compat com ApplySSHConfig
	if a.KeyID != "" {
		if idx := findKeyIndex(state.Keys, a.KeyID); idx >= 0 {
			a.IdentityFile = state.Keys[idx].PrivateKeyPath
		}
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
	// Preserva legado se a nova conta não especifica chave
	if a.KeyID == "" {
		a.KeyID = old.KeyID
		a.IdentityFile = old.IdentityFile
		a.KeyType = old.KeyType
	}
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
			Keys:             state.Keys,
			Errors:           mapValidation(errs),
			SubmitURL:        "/tools/ssh/accounts/" + id,
			IsEdit:           true,
			ShowAliasWarning: showAliasWarning,
		})
		return
	}

	// Resolver chave (somente se keyMode estiver no form)
	if r.FormValue("keyMode") != "" {
		if err := h.resolveAccountKey(r, &a, state); err != nil {
			h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
			return
		}
	}

	// Atualizar IdentityFile para compat com ApplySSHConfig
	if a.KeyID != "" {
		if kidx := findKeyIndex(state.Keys, a.KeyID); kidx >= 0 {
			a.IdentityFile = state.Keys[kidx].PrivateKeyPath
		}
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
	if target.KeyID != "" {
		h.app.Logger.Info("conta removida, chave permanece no Key Manager", "alias", target.HostAlias, "keyID", target.KeyID)
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

func (h *Handler) ApplySSHConfig(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	a, state, ok := h.accountByIDWithState(w, r)
	if !ok {
		return
	}
	// Resolver IdentityFile pelo Key Manager
	if a.KeyID != "" {
		if idx := findKeyIndex(state.Keys, a.KeyID); idx >= 0 {
			a.IdentityFile = state.Keys[idx].PrivateKeyPath
		}
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
	a, state, ok := h.accountByIDWithState(w, r)
	if !ok {
		return
	}
	if a.KeyID != "" {
		if idx := findKeyIndex(state.Keys, a.KeyID); idx >= 0 {
			a.IdentityFile = state.Keys[idx].PrivateKeyPath
		}
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

func (h *Handler) accountByIDWithState(w http.ResponseWriter, r *http.Request) (storage.Account, *storage.State, bool) {
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return storage.Account{}, nil, false
	}
	idx := findAccountIndex(state.Accounts, id)
	if idx < 0 {
		h.operationError(w, "Conta não encontrada", http.StatusNotFound)
		return storage.Account{}, nil, false
	}
	return state.Accounts[idx], state, true
}

// resolveAccountKey lida com seleção ou criação inline de chave.
// Modifica account.KeyID e, se "new", também adiciona Key a state.Keys.
func (h *Handler) resolveAccountKey(r *http.Request, account *storage.Account, state *storage.State) error {
	keyMode := r.FormValue("keyMode")
	switch keyMode {
	case "existing":
		account.KeyID = r.FormValue("keyID")
	case "new":
		keyType := r.FormValue("newKeyType")
		if keyType == "" {
			keyType = "ed25519"
		}
		bits := parseBits(r.FormValue("newKeyBits"))
		comment := strings.TrimSpace(r.FormValue("newKeyComment"))
		if comment == "" {
			comment = account.GitUserEmail
			if comment == "" {
				comment = account.Name
			}
		}
		passphrase := []byte(r.FormValue("newKeyPassphrase"))

		alias := account.HostAlias
		base := alias
		for i := 2; ; i++ {
			conflict := false
			for _, k := range state.Keys {
				if k.Alias == alias {
					conflict = true
					break
				}
			}
			if !conflict {
				break
			}
			alias = fmt.Sprintf("%s-%d", base, i)
		}

		result, err := ssh.GenerateKeyFull(alias, comment, keyType, bits, passphrase, h.app.Paths)
		if err != nil {
			return err
		}

		k := storage.Key{
			ID:             newID(),
			Name:           account.Name,
			Alias:          alias,
			Type:           keyType,
			Bits:           bits,
			Comment:        comment,
			Protected:      len(passphrase) > 0,
			PrivateKeyPath: result.PrivateKeyPath,
			PublicKeyPath:  result.PublicKeyPath,
			Source:         "generated",
			CreatedAt:      time.Now(),
		}
		state.Keys = append(state.Keys, k)
		account.KeyID = k.ID
	}
	return nil
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
		KeyID:        r.FormValue("keyID"),
	}
}

func newID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func buildSSHPreview(a storage.Account, identityPath string) string {
	if a.HostName == "" {
		return "# Configure HostName para usar com git clone via SSH alias"
	}
	if identityPath == "" {
		identityPath = "~/.fdev/keys/" + a.HostAlias + "/id_ed25519"
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
	case "rsa", "rsa4096":
		return "RSA"
	case "ecdsa":
		return "ECDSA"
	default:
		return "Ed25519"
	}
}
