package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/gitconfig"
	"github.com/seuusuario/factorydev/internal/storage"
)

type identityView struct {
	storage.GitIdentity
	KeyName    string
	KeyPubPath string
}

func (h *Handler) globalConfigPath() string {
	return filepath.Join(h.app.Paths.Home, ".gitconfig")
}

// GET /tools/git
func (h *Handler) ListIdentities(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	keyMap := make(map[string]storage.Key, len(state.Keys))
	for _, k := range state.Keys {
		keyMap[k.ID] = k
	}

	views := make([]identityView, len(state.Identities))
	for i, id := range state.Identities {
		v := identityView{GitIdentity: id}
		if k, ok := keyMap[id.KeyID]; ok {
			v.KeyName = k.Name
			v.KeyPubPath = k.PublicKeyPath
		}
		views[i] = v
	}

	globalCfg, _ := gitconfig.ParseGlobalConfig(h.globalConfigPath())
	rules, _ := gitconfig.ListIncludeIf(h.globalConfigPath())

	payload := map[string]any{
		"Identities": views,
		"GlobalCfg":  globalCfg,
		"Rules":      rules,
		"Keys":       state.Keys,
	}

	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "git/list.html", payload)
		return
	}
	h.render(w, "git/list.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "git",
		ContentTpl: "git/list.html",
		Data:       payload,
	})
}

// GET /tools/git/identities/new
func (h *Handler) NewIdentityDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.renderDrawer(w, "Nova Identidade Git", "git/identity-drawer.html", map[string]any{
		"Keys":      state.Keys,
		"SubmitURL": "/tools/git/identities",
		"IsEdit":    false,
	})
}

// POST /tools/git/identities
func (h *Handler) CreateIdentity(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	keyID := strings.TrimSpace(r.FormValue("keyID"))

	if name == "" || email == "" {
		h.errorToast(w, "Nome e e-mail são obrigatórios")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	state.Identities = append(state.Identities, storage.GitIdentity{
		ID:        newID(),
		Name:      name,
		Email:     email,
		KeyID:     keyID,
		CreatedAt: time.Now(),
	})
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Identidade criada!")
	h.ListIdentities(w, r)
}

// GET /tools/git/identities/{id}/edit
func (h *Handler) EditIdentityDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var found *storage.GitIdentity
	for i := range state.Identities {
		if state.Identities[i].ID == id {
			found = &state.Identities[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Identidade não encontrada", http.StatusNotFound)
		return
	}
	h.renderDrawer(w, "Editar Identidade", "git/identity-drawer.html", map[string]any{
		"Identity":  found,
		"Keys":      state.Keys,
		"SubmitURL": "/tools/git/identities/" + id,
		"IsEdit":    true,
	})
}

// POST /tools/git/identities/{id}
func (h *Handler) UpdateIdentity(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	keyID := strings.TrimSpace(r.FormValue("keyID"))

	if name == "" || email == "" {
		h.errorToast(w, "Nome e e-mail são obrigatórios")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	for i := range state.Identities {
		if state.Identities[i].ID == id {
			state.Identities[i].Name = name
			state.Identities[i].Email = email
			state.Identities[i].KeyID = keyID
			break
		}
	}
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Identidade atualizada!")
	h.ListIdentities(w, r)
}

// DELETE /tools/git/identities/{id}
func (h *Handler) DeleteIdentity(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i, ident := range state.Identities {
		if ident.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		h.operationError(w, "Identidade não encontrada", http.StatusNotFound)
		return
	}
	state.Identities = append(state.Identities[:idx], state.Identities[idx+1:]...)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Identidade removida!")
	h.ListIdentities(w, r)
}

// GET /tools/git/global-config
func (h *Handler) GlobalConfigDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	cfg, _ := gitconfig.ParseGlobalConfig(h.globalConfigPath())
	h.renderDrawer(w, "Configuração Git Global", "git/global-config.html", map[string]any{
		"Cfg": cfg,
	})
}

// POST /tools/git/global-config
func (h *Handler) SaveGlobalConfig(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	kvs := map[string]string{
		"user.name":  strings.TrimSpace(r.FormValue("userName")),
		"user.email": strings.TrimSpace(r.FormValue("userEmail")),
	}
	if v := strings.TrimSpace(r.FormValue("gpgSign")); v == "true" || v == "1" {
		kvs["commit.gpgsign"] = "true"
	}
	if v := strings.TrimSpace(r.FormValue("gpgFormat")); v != "" {
		kvs["gpg.format"] = v
	}
	if v := strings.TrimSpace(r.FormValue("signingKey")); v != "" {
		kvs["user.signingkey"] = v
	}

	for k, v := range kvs {
		if v == "" {
			continue
		}
		if err := gitconfig.SetGlobalValue(k, v); err != nil {
			h.errorToast(w, "Erro ao salvar "+k+": "+err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
	}
	h.successToast(w, "Configuração global salva!")
}

// GET /tools/git/includeif
func (h *Handler) IncludeIfDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	rules, _ := gitconfig.ListIncludeIf(h.globalConfigPath())
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.renderDrawer(w, "Regras includeIf", "git/includeif-drawer.html", map[string]any{
		"Rules":      rules,
		"Identities": state.Identities,
	})
}

// POST /tools/git/includeif
func (h *Handler) AddIncludeIfRule(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	dirPath := strings.TrimSpace(r.FormValue("dirPath"))
	includePath := strings.TrimSpace(r.FormValue("includePath"))
	if dirPath == "" || includePath == "" {
		h.errorToast(w, "Diretório e arquivo de config são obrigatórios")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	// Garante trailing slash para gitdir: pattern
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}
	rule := gitconfig.IncludeIfRule{
		Pattern:     "gitdir:" + dirPath,
		IncludePath: includePath,
	}
	if err := gitconfig.AddIncludeIf(h.globalConfigPath(), rule); err != nil {
		h.errorToast(w, err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToast(w, "Regra includeIf adicionada!")
}

// DELETE /tools/git/includeif
func (h *Handler) RemoveIncludeIfRule(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		h.operationError(w, "Parâmetro pattern é obrigatório", http.StatusBadRequest)
		return
	}
	if err := gitconfig.RemoveIncludeIf(h.globalConfigPath(), pattern); err != nil {
		h.errorToast(w, err.Error())
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	h.successToast(w, "Regra removida!")
}

// GET /tools/git/identities/{id}/signing
func (h *Handler) SigningSetupDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var ident *storage.GitIdentity
	for i := range state.Identities {
		if state.Identities[i].ID == id {
			ident = &state.Identities[i]
			break
		}
	}
	if ident == nil {
		h.operationError(w, "Identidade não encontrada", http.StatusNotFound)
		return
	}

	var pubKey, keyName string
	for _, k := range state.Keys {
		if k.ID == ident.KeyID {
			keyName = k.Name
			if data, err2 := readFileSafe(k.PublicKeyPath); err2 == nil {
				pubKey = strings.TrimSpace(data)
			}
			break
		}
	}

	h.renderDrawer(w, "Configurar Signing", "git/signing-setup.html", map[string]any{
		"Identity": ident,
		"KeyName":  keyName,
		"PubKey":   pubKey,
	})
}

// POST /tools/git/identities/{id}/signing
func (h *Handler) ApplySigning(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var ident *storage.GitIdentity
	for i := range state.Identities {
		if state.Identities[i].ID == id {
			ident = &state.Identities[i]
			break
		}
	}
	if ident == nil {
		h.operationError(w, "Identidade não encontrada", http.StatusNotFound)
		return
	}

	var keyPubPath string
	for _, k := range state.Keys {
		if k.ID == ident.KeyID {
			keyPubPath = k.PublicKeyPath
			break
		}
	}
	if keyPubPath == "" {
		h.errorToast(w, "Identidade não possui chave vinculada")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	kvs := map[string]string{
		"gpg.format":      "ssh",
		"user.signingkey": keyPubPath,
		"commit.gpgsign":  "true",
	}
	for k, v := range kvs {
		if err := gitconfig.SetGlobalValue(k, v); err != nil {
			h.errorToast(w, "Erro: "+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	h.successToast(w, "Commit signing configurado!")
}

func readFileSafe(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
