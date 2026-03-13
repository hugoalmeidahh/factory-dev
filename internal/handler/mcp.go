package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/storage"
)

// ── Dashboard (tabs: Servers | Skills) ───────────────────────────

func (h *Handler) MCPDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	data := PageData{
		Title:      "MCP & Skills",
		ActiveTool: "mcp",
		ContentTpl: "mcp/dashboard.html",
		Data: map[string]any{
			"Servers": st.MCPServers,
			"Skills":  st.CustomSkills,
		},
	}
	h.render(w, "mcp/dashboard.html", data)
}

// ── MCP Servers CRUD ─────────────────────────────────────────────

func (h *Handler) NewMCPServerDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Novo MCP Server", "mcp/server-drawer.html", map[string]any{
		"Server": storage.MCPServer{Enabled: true},
		"EnvJSON": "[]",
	})
}

func (h *Handler) CreateMCPServer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	srv := parseMCPServerForm(r)
	srv.ID = newID()
	srv.CreatedAt = time.Now()

	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	st.MCPServers = append(st.MCPServers, srv)
	if err := h.app.Storage.SaveState(st); err != nil {
		h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
		return
	}
	h.successToast(w, "MCP Server criado")
}

func (h *Handler) EditMCPServerDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, _ := h.app.Storage.LoadState()
	for _, s := range st.MCPServers {
		if s.ID == id {
			h.renderDrawer(w, "Editar MCP Server", "mcp/server-drawer.html", map[string]any{
				"Server":  s,
				"EnvJSON": envVarsToJSON(s.Env),
			})
			return
		}
	}
	h.operationError(w, "Server não encontrado", http.StatusNotFound)
}

func (h *Handler) UpdateMCPServer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	for i, s := range st.MCPServers {
		if s.ID == id {
			updated := parseMCPServerForm(r)
			updated.ID = s.ID
			updated.CreatedAt = s.CreatedAt
			st.MCPServers[i] = updated
			if err := h.app.Storage.SaveState(st); err != nil {
				h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
				return
			}
			h.successToast(w, "MCP Server atualizado")
			return
		}
	}
	h.operationError(w, "Server não encontrado", http.StatusNotFound)
}

func (h *Handler) DeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	for i, s := range st.MCPServers {
		if s.ID == id {
			st.MCPServers = append(st.MCPServers[:i], st.MCPServers[i+1:]...)
			if err := h.app.Storage.SaveState(st); err != nil {
				h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
				return
			}
			h.successToast(w, "MCP Server removido")
			return
		}
	}
	h.operationError(w, "Server não encontrado", http.StatusNotFound)
}

// SyncToClaudeCode grava os MCP servers no ~/.claude/settings.json.
func (h *Handler) SyncToClaudeCode(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}

	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	// Lê settings existente ou cria novo
	settings := make(map[string]any)
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	// Converte MCP servers para o formato do Claude Code
	mcpServers := make(map[string]any)
	for _, s := range st.MCPServers {
		if !s.Enabled {
			continue
		}
		entry := map[string]any{
			"command": s.Command,
		}
		if len(s.Args) > 0 {
			entry["args"] = s.Args
		}
		if len(s.Env) > 0 {
			entry["env"] = s.Env
		}
		mcpServers[s.Name] = entry
	}
	settings["mcpServers"] = mcpServers

	// Garante que o diretório existe
	_ = os.MkdirAll(filepath.Dir(settingsPath), 0o700)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		h.operationError(w, "Erro ao serializar JSON", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		h.operationError(w, "Erro ao gravar settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.successToastOnly(w, "Sync OK! MCP servers gravados em ~/.claude/settings.json")
}

// ── Custom Skills CRUD ───────────────────────────────────────────

func (h *Handler) NewSkillDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Nova Skill", "mcp/skill-drawer.html", map[string]any{
		"Skill": storage.CustomSkill{},
	})
}

func (h *Handler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	skill := parseSkillForm(r)
	skill.ID = newID()
	skill.CreatedAt = time.Now()

	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	st.CustomSkills = append(st.CustomSkills, skill)
	if err := h.app.Storage.SaveState(st); err != nil {
		h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Skill criada")
}

func (h *Handler) EditSkillDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, _ := h.app.Storage.LoadState()
	for _, s := range st.CustomSkills {
		if s.ID == id {
			h.renderDrawer(w, "Editar Skill", "mcp/skill-drawer.html", map[string]any{
				"Skill": s,
			})
			return
		}
	}
	h.operationError(w, "Skill não encontrada", http.StatusNotFound)
}

func (h *Handler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	for i, s := range st.CustomSkills {
		if s.ID == id {
			updated := parseSkillForm(r)
			updated.ID = s.ID
			updated.CreatedAt = s.CreatedAt
			st.CustomSkills[i] = updated
			if err := h.app.Storage.SaveState(st); err != nil {
				h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
				return
			}
			h.successToast(w, "Skill atualizada")
			return
		}
	}
	h.operationError(w, "Skill não encontrada", http.StatusNotFound)
}

func (h *Handler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	for i, s := range st.CustomSkills {
		if s.ID == id {
			st.CustomSkills = append(st.CustomSkills[:i], st.CustomSkills[i+1:]...)
			if err := h.app.Storage.SaveState(st); err != nil {
				h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
				return
			}
			h.successToast(w, "Skill removida")
			return
		}
	}
	h.operationError(w, "Skill não encontrada", http.StatusNotFound)
}

// CopySkillPrompt retorna o prompt da skill para o clipboard (via HX-Trigger).
func (h *Handler) CopySkillPrompt(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, _ := h.app.Storage.LoadState()
	for _, s := range st.CustomSkills {
		if s.ID == id {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(s.Prompt))
			return
		}
	}
	h.operationError(w, "Skill não encontrada", http.StatusNotFound)
}

// ── Helpers ──────────────────────────────────────────────────────

func parseMCPServerForm(r *http.Request) storage.MCPServer {
	var args []string
	argsRaw := strings.TrimSpace(r.FormValue("args"))
	if argsRaw != "" {
		// Tenta JSON array, senão split por newline
		if err := json.Unmarshal([]byte(argsRaw), &args); err != nil {
			for _, line := range strings.Split(argsRaw, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					args = append(args, line)
				}
			}
		}
	}

	env := make(map[string]string)
	envJSON := r.FormValue("envJSON")
	if envJSON != "" {
		_ = json.Unmarshal([]byte(envJSON), &env)
	}

	return storage.MCPServer{
		Name:    strings.TrimSpace(r.FormValue("name")),
		Command: strings.TrimSpace(r.FormValue("command")),
		Args:    args,
		Env:     env,
		Enabled: r.FormValue("enabled") == "on" || r.FormValue("enabled") == "true",
	}
}

func parseSkillForm(r *http.Request) storage.CustomSkill {
	var tags []string
	tagsRaw := strings.TrimSpace(r.FormValue("tags"))
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}
	return storage.CustomSkill{
		Name:        strings.TrimSpace(r.FormValue("name")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Prompt:      r.FormValue("prompt"),
		Tags:        tags,
	}
}
