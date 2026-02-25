package handler

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/ssh"
	"github.com/seuusuario/factorydev/internal/storage"
)

type keyView struct {
	storage.Key
	HasPrivKey        bool
	HasPubKey         bool
	PublicKeyContent  string
	PrivateKeyContent string
	UsedByAccounts    []string
}

type keyFormData struct {
	Key    storage.Key
	Errors map[string]string
}

func (h *Handler) ListKeys(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	// Mapa de uso: keyID → nomes das accounts que referenciam
	keyUsage := make(map[string][]string)
	for _, a := range state.Accounts {
		if a.KeyID != "" {
			keyUsage[a.KeyID] = append(keyUsage[a.KeyID], a.Name)
		}
	}

	views := make([]keyView, 0, len(state.Keys))
	for _, k := range state.Keys {
		_, privErr := os.Stat(k.PrivateKeyPath)
		_, pubErr := os.Stat(k.PublicKeyPath)
		var pubContent, privContent string
		if pubErr == nil {
			b, _ := os.ReadFile(k.PublicKeyPath)
			pubContent = strings.TrimSpace(string(b))
		}
		if privErr == nil {
			b, _ := os.ReadFile(k.PrivateKeyPath)
			privContent = strings.TrimSpace(string(b))
		}
		views = append(views, keyView{
			Key:               k,
			HasPrivKey:        privErr == nil,
			HasPubKey:         pubErr == nil,
			PublicKeyContent:  pubContent,
			PrivateKeyContent: privContent,
			UsedByAccounts:    keyUsage[k.ID],
		})
	}

	data := map[string]any{"Keys": views}
	pageData := PageData{
		Title:      "Key Manager — FactoryDev",
		ActiveTool: "keys",
		ContentTpl: "keys/list.html",
		Data:       data,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "keys/list.html", data)
		return
	}
	h.render(w, "keys/list.html", pageData)
}

func (h *Handler) NewKeyDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Nova Chave", "keys/key-drawer.html", keyFormData{
		Key:    storage.Key{Type: "ed25519"},
		Errors: map[string]string{},
	})
}

func (h *Handler) CreateKey(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	alias := strings.TrimSpace(r.FormValue("alias"))
	if alias == "" {
		alias = sanitizeAlias(name)
	}
	keyType := r.FormValue("keyType")
	if keyType == "" {
		keyType = "ed25519"
	}
	bits := parseBits(r.FormValue("bits"))
	comment := strings.TrimSpace(r.FormValue("comment"))
	passphrase := []byte(r.FormValue("passphrase"))

	k := storage.Key{
		ID:        newID(),
		Name:      name,
		Alias:     alias,
		Type:      keyType,
		Bits:      bits,
		Comment:   comment,
		Protected: len(passphrase) > 0,
		Source:    "generated",
		CreatedAt: time.Now(),
	}

	if errs := storage.ValidateKey(k); len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderDrawer(w, "Nova Chave", "keys/key-drawer.html", keyFormData{
			Key:    k,
			Errors: mapValidation(errs),
		})
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	// Verificar unicidade do alias
	for _, existing := range state.Keys {
		if existing.Alias == alias {
			w.WriteHeader(http.StatusUnprocessableEntity)
			h.renderDrawer(w, "Nova Chave", "keys/key-drawer.html", keyFormData{
				Key:    k,
				Errors: map[string]string{"alias": "alias já existe"},
			})
			return
		}
	}

	result, err := ssh.GenerateKeyFull(alias, comment, keyType, bits, passphrase, h.app.Paths)
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	k.PrivateKeyPath = result.PrivateKeyPath
	k.PublicKeyPath = result.PublicKeyPath

	state.Keys = append(state.Keys, k)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	h.successToast(w, "Chave criada com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListKeys(w, r)
}

func (h *Handler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	// Verificar se a chave está em uso
	var usedBy []string
	for _, a := range state.Accounts {
		if a.KeyID == id {
			usedBy = append(usedBy, a.Name)
		}
	}
	if len(usedBy) > 0 {
		h.operationError(w, "Chave em uso por: "+strings.Join(usedBy, ", "), http.StatusConflict)
		return
	}

	idx := findKeyIndex(state.Keys, id)
	if idx < 0 {
		h.operationError(w, "Chave não encontrada", http.StatusNotFound)
		return
	}

	state.Keys = append(state.Keys[:idx], state.Keys[idx+1:]...)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	h.successToast(w, "Chave removida com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListKeys(w, r)
}

func (h *Handler) RegenPublicKey(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	idx := findKeyIndex(state.Keys, id)
	if idx < 0 {
		h.operationError(w, "Chave não encontrada", http.StatusNotFound)
		return
	}
	k := state.Keys[idx]
	passphrase := []byte(r.FormValue("passphrase"))

	if err := ssh.RegenPublicKey(k.PrivateKeyPath, k.PublicKeyPath, passphrase); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusBadRequest)
		return
	}

	h.successToast(w, "Chave pública regenerada com sucesso!")
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListKeys(w, r)
}

func (h *Handler) ExportKeyBase64(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	which := r.URL.Query().Get("which")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	idx := findKeyIndex(state.Keys, id)
	if idx < 0 {
		h.operationError(w, "Chave não encontrada", http.StatusNotFound)
		return
	}
	k := state.Keys[idx]

	var path string
	if which == "priv" {
		path = k.PrivateKeyPath
	} else {
		path = k.PublicKeyPath
		which = "pub"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		h.operationError(w, "Não foi possível ler a chave", http.StatusInternalServerError)
		return
	}

	h.renderDrawer(w, "Exportar Chave — Base64", "keys/export-result.html", map[string]any{
		"B64":   base64.StdEncoding.EncodeToString(data),
		"Which": which,
		"Name":  k.Name,
	})
}

func (h *Handler) ImportKeysDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	home, _ := os.UserHomeDir()
	h.renderDrawer(w, "Importar Chaves", "keys/import-drawer.html", map[string]any{
		"Dir": home + "/.ssh",
	})
}

func (h *Handler) ValidateImportPath(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	dir := strings.TrimSpace(r.FormValue("dir"))
	candidates, err := ssh.ScanDir(dir)
	if err != nil {
		h.render(w, "keys/import-preview.html", map[string]any{
			"Error": "Não foi possível ler o diretório: " + err.Error(),
			"Dir":   dir,
		})
		return
	}
	h.render(w, "keys/import-preview.html", map[string]any{
		"Candidates": candidates,
		"Dir":        dir,
	})
}

func (h *Handler) ImportKeys(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	dir := strings.TrimSpace(r.FormValue("dir"))
	selected := r.Form["selected"]
	if len(selected) == 0 {
		h.operationError(w, "Selecione ao menos uma chave para importar", http.StatusBadRequest)
		return
	}

	candidates, err := ssh.ScanDir(dir)
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusBadRequest)
		return
	}

	selectedSet := make(map[string]bool)
	for _, s := range selected {
		selectedSet[s] = true
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	imported := 0
	for _, c := range candidates {
		if !selectedSet[c.Name] {
			continue
		}
		// Alias único baseado no nome do arquivo
		alias := sanitizeAlias(c.Name)
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

		privDest, pubDest, err := ssh.CopyKeyPair(c, alias, h.app.Paths)
		if err != nil {
			h.app.Logger.Warn("falha ao copiar chave", "name", c.Name, "err", err)
			continue
		}

		k := storage.Key{
			ID:             newID(),
			Name:           c.Name,
			Alias:          alias,
			Type:           c.Type,
			Bits:           c.Bits,
			Protected:      c.Protected,
			PrivateKeyPath: privDest,
			PublicKeyPath:  pubDest,
			Source:         "imported",
			OriginalPath:   c.PrivatePath,
			CreatedAt:      time.Now(),
		}
		state.Keys = append(state.Keys, k)
		imported++
	}

	if imported == 0 {
		h.operationError(w, "Nenhuma chave foi importada", http.StatusInternalServerError)
		return
	}

	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	h.successToast(w, fmt.Sprintf("%d chave(s) importada(s) com sucesso!", imported))
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListKeys(w, r)
}

// ── Helpers ───────────────────────────────────────────────────────

func findKeyIndex(keys []storage.Key, id string) int {
	for i := range keys {
		if keys[i].ID == id {
			return i
		}
	}
	return -1
}

func parseBits(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
