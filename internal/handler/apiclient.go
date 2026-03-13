package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/storage"
)

// GET /tools/api
func (h *Handler) APIDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	// Agrupar endpoints por collection
	type collectionWithEndpoints struct {
		storage.APICollection
		Endpoints []storage.APIEndpoint
	}
	var collections []collectionWithEndpoints
	for _, c := range state.APICollections {
		cwe := collectionWithEndpoints{APICollection: c}
		for _, e := range state.APIEndpoints {
			if e.CollectionID == c.ID {
				cwe.Endpoints = append(cwe.Endpoints, e)
			}
		}
		collections = append(collections, cwe)
	}

	payload := map[string]any{
		"Collections": collections,
		"History":     limitHistory(state.APIHistory, 20),
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "apiclient/dashboard.html", payload)
		return
	}
	h.render(w, "apiclient/dashboard.html", PageData{
		Title:      "FactoryDev",
		ActiveTool: "api",
		ContentTpl: "apiclient/dashboard.html",
		Data:       payload,
	})
}

// GET /tools/api/collections/new
func (h *Handler) NewCollectionDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	h.renderDrawer(w, "Nova Collection", "apiclient/collection-drawer.html", map[string]any{
		"SubmitURL":  "/tools/api/collections",
		"IsEdit":     false,
		"Collection": storage.APICollection{AuthType: "none"},
		"EnvJSON":    "[]",
		"AuthJSON":   "{}",
	})
}

// POST /tools/api/collections
func (h *Handler) CreateCollection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	c := parseCollectionForm(r)
	c.ID = newID()
	c.CreatedAt = time.Now()

	if strings.TrimSpace(c.Name) == "" {
		h.errorToast(w, "Nome é obrigatório")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	state.APICollections = append(state.APICollections, c)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Collection criada!")
}

// GET /tools/api/collections/{id}/edit
func (h *Handler) EditCollectionDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var found *storage.APICollection
	for i := range state.APICollections {
		if state.APICollections[i].ID == id {
			found = &state.APICollections[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Collection não encontrada", http.StatusNotFound)
		return
	}
	envJSON := envVarsToJSON(found.EnvVars)
	authJSON, _ := json.Marshal(found.AuthData)
	h.renderDrawer(w, "Editar Collection", "apiclient/collection-drawer.html", map[string]any{
		"SubmitURL":  "/tools/api/collections/" + id,
		"IsEdit":     true,
		"Collection": found,
		"EnvJSON":    envJSON,
		"AuthJSON":   string(authJSON),
	})
}

// POST /tools/api/collections/{id}
func (h *Handler) UpdateCollection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	c := parseCollectionForm(r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	for i := range state.APICollections {
		if state.APICollections[i].ID == id {
			state.APICollections[i].Name = c.Name
			state.APICollections[i].AuthType = c.AuthType
			state.APICollections[i].AuthData = c.AuthData
			state.APICollections[i].EnvVars = c.EnvVars
			break
		}
	}
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Collection atualizada!")
}

// DELETE /tools/api/collections/{id}
func (h *Handler) DeleteCollection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i, c := range state.APICollections {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		h.operationError(w, "Collection não encontrada", http.StatusNotFound)
		return
	}
	state.APICollections = append(state.APICollections[:idx], state.APICollections[idx+1:]...)
	// Remover endpoints da collection
	var kept []storage.APIEndpoint
	for _, e := range state.APIEndpoints {
		if e.CollectionID != id {
			kept = append(kept, e)
		}
	}
	state.APIEndpoints = kept
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Collection removida!")
}

// GET /tools/api/endpoints/new
func (h *Handler) NewEndpointDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	collectionID := r.URL.Query().Get("collectionId")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.renderDrawer(w, "Novo Endpoint", "apiclient/endpoint-drawer.html", map[string]any{
		"SubmitURL":    "/tools/api/endpoints",
		"IsEdit":       false,
		"Endpoint":     storage.APIEndpoint{CollectionID: collectionID, Method: "GET"},
		"Collections":  state.APICollections,
		"HeadersJSON":  "[]",
	})
}

// POST /tools/api/endpoints
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	ep := parseEndpointForm(r)
	ep.ID = newID()
	ep.CreatedAt = time.Now()
	ep.UpdatedAt = time.Now()

	if strings.TrimSpace(ep.Name) == "" || strings.TrimSpace(ep.URL) == "" {
		h.errorToast(w, "Nome e URL são obrigatórios")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	state.APIEndpoints = append(state.APIEndpoints, ep)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Endpoint criado!")
}

// GET /tools/api/endpoints/{id}/edit
func (h *Handler) EditEndpointDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	var found *storage.APIEndpoint
	for i := range state.APIEndpoints {
		if state.APIEndpoints[i].ID == id {
			found = &state.APIEndpoints[i]
			break
		}
	}
	if found == nil {
		h.operationError(w, "Endpoint não encontrado", http.StatusNotFound)
		return
	}
	headersJSON := envVarsToJSON(found.Headers)
	h.renderDrawer(w, "Editar Endpoint", "apiclient/endpoint-drawer.html", map[string]any{
		"SubmitURL":   "/tools/api/endpoints/" + id,
		"IsEdit":      true,
		"Endpoint":    found,
		"Collections": state.APICollections,
		"HeadersJSON": headersJSON,
	})
}

// POST /tools/api/endpoints/{id}
func (h *Handler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}
	ep := parseEndpointForm(r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	for i := range state.APIEndpoints {
		if state.APIEndpoints[i].ID == id {
			state.APIEndpoints[i].Name = ep.Name
			state.APIEndpoints[i].CollectionID = ep.CollectionID
			state.APIEndpoints[i].Method = ep.Method
			state.APIEndpoints[i].URL = ep.URL
			state.APIEndpoints[i].Headers = ep.Headers
			state.APIEndpoints[i].Body = ep.Body
			state.APIEndpoints[i].UpdatedAt = time.Now()
			break
		}
	}
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Endpoint atualizado!")
}

// DELETE /tools/api/endpoints/{id}
func (h *Handler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	idx := -1
	for i, e := range state.APIEndpoints {
		if e.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		h.operationError(w, "Endpoint não encontrado", http.StatusNotFound)
		return
	}
	state.APIEndpoints = append(state.APIEndpoints[:idx], state.APIEndpoints[idx+1:]...)
	if err := h.app.Storage.SaveState(state); err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Endpoint removido!")
}

// POST /tools/api/endpoints/{id}/send
func (h *Handler) SendRequest(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}

	var ep *storage.APIEndpoint
	for i := range state.APIEndpoints {
		if state.APIEndpoints[i].ID == id {
			ep = &state.APIEndpoints[i]
			break
		}
	}
	if ep == nil {
		h.operationError(w, "Endpoint não encontrado", http.StatusNotFound)
		return
	}

	// Encontrar collection para auth e envs
	var col *storage.APICollection
	for i := range state.APICollections {
		if state.APICollections[i].ID == ep.CollectionID {
			col = &state.APICollections[i]
			break
		}
	}

	result := executeHTTPRequest(*ep, col)

	// Salvar no history
	hist := storage.APIRequestHistory{
		ID:           newID(),
		EndpointID:   ep.ID,
		Method:       ep.Method,
		URL:          result.FinalURL,
		StatusCode:   result.StatusCode,
		ResponseTime: result.DurationMs,
		RequestedAt:  time.Now(),
	}
	state.APIHistory = append(state.APIHistory, hist)
	if len(state.APIHistory) > 100 {
		state.APIHistory = state.APIHistory[len(state.APIHistory)-100:]
	}
	_ = h.app.Storage.SaveState(state)

	h.render(w, "apiclient/response.html", map[string]any{
		"Result": result,
	})
}

// POST /tools/api/send (ad-hoc)
func (h *Handler) SendAdHocRequest(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	if err := r.ParseForm(); err != nil {
		h.operationError(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	ep := storage.APIEndpoint{
		Method: strings.TrimSpace(r.FormValue("method")),
		URL:    strings.TrimSpace(r.FormValue("url")),
		Body:   r.FormValue("body"),
	}
	if ep.URL == "" {
		h.errorToast(w, "URL é obrigatória")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	if ep.Method == "" {
		ep.Method = "GET"
	}

	result := executeHTTPRequest(ep, nil)
	h.render(w, "apiclient/response.html", map[string]any{
		"Result": result,
	})
}

// GET /tools/api/history
func (h *Handler) APIRequestHistory(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	h.render(w, "apiclient/history.html", map[string]any{
		"History": limitHistory(state.APIHistory, 50),
	})
}

// ── helpers ────────────────────────────────────────────────────────

type httpResult struct {
	FinalURL    string
	Method      string
	StatusCode  int
	StatusText  string
	DurationMs  int64
	Headers     map[string]string
	Body        string
	Error       string
	IsJSON      bool
}

func executeHTTPRequest(ep storage.APIEndpoint, col *storage.APICollection) httpResult {
	result := httpResult{Method: ep.Method}

	// Substituir variáveis {{VAR}} com envVars da collection
	url := ep.URL
	body := ep.Body
	if col != nil {
		for k, v := range col.EnvVars {
			placeholder := "{{" + k + "}}"
			url = strings.ReplaceAll(url, placeholder, v)
			body = strings.ReplaceAll(body, placeholder, v)
		}
	}
	result.FinalURL = url

	// Criar request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(ep.Method, url, bodyReader)
	if err != nil {
		result.Error = fmt.Sprintf("Erro ao criar request: %v", err)
		return result
	}

	// Headers do endpoint
	for k, v := range ep.Headers {
		if col != nil {
			for ek, ev := range col.EnvVars {
				v = strings.ReplaceAll(v, "{{"+ek+"}}", ev)
			}
		}
		req.Header.Set(k, v)
	}

	// Auth da collection
	if col != nil {
		switch col.AuthType {
		case "bearer":
			if token := col.AuthData["token"]; token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "basic":
			if user := col.AuthData["username"]; user != "" {
				req.SetBasicAuth(user, col.AuthData["password"])
			}
		case "apikey":
			headerName := col.AuthData["headerName"]
			if headerName == "" {
				headerName = "X-API-Key"
			}
			if key := col.AuthData["headerValue"]; key != "" {
				req.Header.Set(headerName, key)
			}
		}
	}

	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	result.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = fmt.Sprintf("Erro na requisição: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.StatusText = resp.Status

	// Response headers
	result.Headers = make(map[string]string)
	for k := range resp.Header {
		result.Headers[k] = resp.Header.Get(k)
	}

	// Response body
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		result.Error = "Erro ao ler response body"
		return result
	}

	// Try pretty-print JSON
	var pretty bytes.Buffer
	if json.Indent(&pretty, respBody, "", "  ") == nil {
		result.Body = pretty.String()
		result.IsJSON = true
	} else {
		result.Body = string(respBody)
	}

	return result
}

func parseCollectionForm(r *http.Request) storage.APICollection {
	c := storage.APICollection{
		Name:     strings.TrimSpace(r.FormValue("name")),
		AuthType: strings.TrimSpace(r.FormValue("authType")),
	}
	if c.AuthType == "" {
		c.AuthType = "none"
	}

	// Auth data
	c.AuthData = make(map[string]string)
	switch c.AuthType {
	case "bearer":
		c.AuthData["token"] = strings.TrimSpace(r.FormValue("authToken"))
	case "basic":
		c.AuthData["username"] = strings.TrimSpace(r.FormValue("authUsername"))
		c.AuthData["password"] = r.FormValue("authPassword")
	case "apikey":
		c.AuthData["headerName"] = strings.TrimSpace(r.FormValue("authHeaderName"))
		c.AuthData["headerValue"] = strings.TrimSpace(r.FormValue("authHeaderValue"))
	}

	// Env vars
	c.EnvVars = parseJSONKeyValue(r.FormValue("envVars"))
	return c
}

func parseEndpointForm(r *http.Request) storage.APIEndpoint {
	return storage.APIEndpoint{
		CollectionID: strings.TrimSpace(r.FormValue("collectionId")),
		Name:         strings.TrimSpace(r.FormValue("name")),
		Method:       strings.TrimSpace(r.FormValue("method")),
		URL:          strings.TrimSpace(r.FormValue("url")),
		Headers:      parseJSONKeyValue(r.FormValue("headers")),
		Body:         r.FormValue("body"),
	}
}

func parseJSONKeyValue(raw string) map[string]string {
	result := make(map[string]string)
	if raw == "" {
		return result
	}
	var pairs []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(raw), &pairs); err == nil {
		for _, p := range pairs {
			k := strings.TrimSpace(p.Key)
			if k != "" {
				result[k] = p.Value
			}
		}
	}
	return result
}

func limitHistory(h []storage.APIRequestHistory, n int) []storage.APIRequestHistory {
	if len(h) <= n {
		// Reverse for newest first
		out := make([]storage.APIRequestHistory, len(h))
		for i, v := range h {
			out[len(h)-1-i] = v
		}
		return out
	}
	out := make([]storage.APIRequestHistory, n)
	for i := 0; i < n; i++ {
		out[i] = h[len(h)-1-i]
	}
	return out
}

// statusColor retorna a cor CSS baseada no status code.
func statusColor(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "var(--success)"
	case code >= 300 && code < 400:
		return "var(--warning, orange)"
	default:
		return "var(--danger)"
	}
}

func init() {
	tmplFuncs["statusColor"] = statusColor
	tmplFuncs["sortedKeys"] = func(m map[string]string) []string {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}
}
