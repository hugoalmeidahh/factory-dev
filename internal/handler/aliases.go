package handler

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/storage"
)

// GET /tools/aliases
func (h *Handler) ListAliases(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	aliases := make([]storage.ShellAlias, len(state.Aliases))
	copy(aliases, state.Aliases)
	sort.Slice(aliases, func(i, j int) bool { return aliases[i].Name < aliases[j].Name })

	payload := map[string]any{
		"Aliases":   aliases,
		"AliasFile": filepath.Join(h.app.Paths.Base, "aliases.sh"),
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "aliases/list.html", payload)
		return
	}
	h.render(w, "aliases/list.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "aliases",
		ContentTpl: "aliases/list.html",
		Data:       payload,
	})
}

// GET /tools/aliases/new
func (h *Handler) NewAliasDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Novo Alias", "aliases/alias-drawer.html", map[string]any{
		"SubmitURL": "/tools/aliases",
		"IsEdit":    false,
		"Alias":     storage.ShellAlias{},
	})
}

// POST /tools/aliases
func (h *Handler) CreateAlias(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	a := parseAliasForm(r)
	a.ID = newID()
	a.CreatedAt = time.Now()

	if errs := storage.ValidateShellAlias(a); len(errs) > 0 {
		h.errorToast(w, errs[0].Message)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	// Verificar duplicata
	for _, existing := range state.Aliases {
		if existing.Name == a.Name {
			h.errorToast(w, "Alias já existe: "+a.Name)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
	}

	state.Aliases = append(state.Aliases, a)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.syncAliasFile(state)
	h.successToast(w, "Alias criado!")
}

// GET /tools/aliases/{id}/edit
func (h *Handler) EditAliasDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var found *storage.ShellAlias
	for i := range state.Aliases {
		if state.Aliases[i].ID == id {
			found = &state.Aliases[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Alias não encontrado", http.StatusNotFound)
		return
	}
	h.renderDrawer(w, "Editar Alias", "aliases/alias-drawer.html", map[string]any{
		"SubmitURL": "/tools/aliases/" + id,
		"IsEdit":    true,
		"Alias":     found,
	})
}

// POST /tools/aliases/{id}
func (h *Handler) UpdateAlias(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	a := parseAliasForm(r)
	if errs := storage.ValidateShellAlias(a); len(errs) > 0 {
		h.errorToast(w, errs[0].Message)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	for i := range state.Aliases {
		if state.Aliases[i].ID == id {
			state.Aliases[i].Name = a.Name
			state.Aliases[i].Command = a.Command
			break
		}
	}
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.syncAliasFile(state)
	h.successToast(w, "Alias atualizado!")
}

// DELETE /tools/aliases/{id}
func (h *Handler) DeleteAlias(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i, a := range state.Aliases {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		h.operationError(w, "Alias não encontrado", http.StatusNotFound)
		return
	}
	state.Aliases = append(state.Aliases[:idx], state.Aliases[idx+1:]...)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.syncAliasFile(state)
	h.successToast(w, "Alias removido!")
}

// ── helpers ────────────────────────────────────────────────────────

func parseAliasForm(r *http.Request) storage.ShellAlias {
	return storage.ShellAlias{
		Name:    strings.TrimSpace(r.FormValue("name")),
		Command: strings.TrimSpace(r.FormValue("command")),
	}
}

// syncAliasFile regenera ~/.fdev/aliases.sh e garante source nos shells.
func (h *Handler) syncAliasFile(state *storage.State) {
	aliasPath := filepath.Join(h.app.Paths.Base, "aliases.sh")

	// Gera conteúdo
	var sb strings.Builder
	sb.WriteString("# Gerado automaticamente pelo FactoryDev — não edite manualmente\n")
	for _, a := range state.Aliases {
		// Escapa aspas simples no comando
		cmd := strings.ReplaceAll(a.Command, "'", "'\\''")
		sb.WriteString(fmt.Sprintf("alias %s='%s'\n", a.Name, cmd))
	}

	if err := os.WriteFile(aliasPath, []byte(sb.String()), 0o644); err != nil {
		h.app.Logger.Error("erro ao gravar aliases.sh", "err", err)
		return
	}

	// Garante source nos shells
	sourceLine := fmt.Sprintf("source %q", aliasPath)
	for _, rc := range []string{
		filepath.Join(h.app.Paths.Home, ".zshrc"),
		filepath.Join(h.app.Paths.Home, ".bashrc"),
	} {
		ensureSourceLine(rc, sourceLine)
	}
}

// ensureSourceLine adiciona a linha de source se não existir no arquivo rc.
func ensureSourceLine(rcPath, sourceLine string) {
	f, err := os.Open(rcPath)
	if err != nil {
		// Arquivo não existe — cria com a linha
		_ = os.WriteFile(rcPath, []byte("# Added by FactoryDev\n"+sourceLine+"\n"), 0o644)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), sourceLine) {
			return // já existe
		}
	}

	// Append
	af, err := os.OpenFile(rcPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer af.Close()
	_, _ = af.WriteString("\n# Added by FactoryDev\n" + sourceLine + "\n")
}
