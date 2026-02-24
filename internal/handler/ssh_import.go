package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/ssh"
	"github.com/seuusuario/factorydev/internal/storage"
)

func (h *Handler) ImportSSHConfigDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	home, _ := os.UserHomeDir()
	h.renderDrawer(w, "Importar SSH Config", "ssh/import-drawer.html", map[string]any{
		"ConfigPath": home + "/.ssh/config",
	})
}

func (h *Handler) ValidateSSHConfigPath(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	configPath := strings.TrimSpace(r.FormValue("configPath"))
	blocks, err := ssh.ParseImportableBlocks(configPath)
	if err != nil {
		h.render(w, "ssh/import-preview.html", map[string]any{
			"Error":      "Não foi possível ler o arquivo: " + err.Error(),
			"ConfigPath": configPath,
		})
		return
	}
	h.render(w, "ssh/import-preview.html", map[string]any{
		"Blocks":     blocks,
		"ConfigPath": configPath,
	})
}

func (h *Handler) ImportSSHAccounts(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	configPath := strings.TrimSpace(r.FormValue("configPath"))
	selected := r.Form["selected"]
	if len(selected) == 0 {
		h.operationError(w, "Selecione ao menos uma conta para importar", http.StatusBadRequest)
		return
	}

	blocks, err := ssh.ParseImportableBlocks(configPath)
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
	for _, block := range blocks {
		if !selectedSet[block.HostAlias] {
			continue
		}
		// Verificar conflito de alias
		conflict := false
		for _, a := range state.Accounts {
			if a.HostAlias == block.HostAlias {
				conflict = true
				break
			}
		}
		if conflict {
			continue
		}

		a := storage.Account{
			ID:           newID(),
			Name:         block.HostAlias,
			Provider:     "other",
			HostName:     block.HostName,
			HostAlias:    block.HostAlias,
			IdentityFile: block.IdentityFile,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		state.Accounts = append(state.Accounts, a)
		imported++
	}

	if imported == 0 {
		h.operationError(w, "Nenhuma conta foi importada (possíveis conflitos de alias)", http.StatusConflict)
		return
	}

	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	h.successToast(w, fmt.Sprintf("%d conta(s) importada(s) com sucesso!", imported))
	w.Header().Set("HX-Retarget", "#main-content")
	h.ListAccounts(w, r)
}
