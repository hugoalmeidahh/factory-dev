package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seuusuario/factorydev/internal/dbclient"
	"github.com/seuusuario/factorydev/internal/storage"
)

// Pool de conexões ativas — gerenciado pelo handler.
var (
	dbConns   = make(map[string]*sql.DB)
	dbConnsMu sync.Mutex
)

func getDBConn(conn storage.DBConnection) (*sql.DB, error) {
	dbConnsMu.Lock()
	defer dbConnsMu.Unlock()

	if db, ok := dbConns[conn.ID]; ok {
		if err := db.Ping(); err == nil {
			return db, nil
		}
		db.Close()
		delete(dbConns, conn.ID)
	}
	db, err := dbclient.Open(conn)
	if err != nil {
		return nil, err
	}
	dbConns[conn.ID] = db
	return db, nil
}

func closeDBConn(id string) {
	dbConnsMu.Lock()
	defer dbConnsMu.Unlock()
	if db, ok := dbConns[id]; ok {
		db.Close()
		delete(dbConns, id)
	}
}

// ── Dashboard ────────────────────────────────────────────────────

func (h *Handler) DBDashboard(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	data := PageData{
		Title:      "Database Browser",
		ActiveTool: "database",
		ContentTpl: "database/dashboard.html",
		Data: map[string]any{
			"Connections": st.DBConnections,
		},
	}
	h.render(w, "database/dashboard.html", data)
}

// ── CRUD Connections ─────────────────────────────────────────────

func (h *Handler) NewDBConnectionDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	st, _ := h.app.Storage.LoadState()
	h.renderDrawer(w, "Nova Conexão", "database/connection-drawer.html", map[string]any{
		"Conn": storage.DBConnection{Port: 5432, Driver: "postgres"},
		"Keys": st.Keys,
	})
}

func (h *Handler) CreateDBConnection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	conn := parseDBConnForm(r)
	conn.ID = newID()
	conn.CreatedAt = time.Now()

	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	st.DBConnections = append(st.DBConnections, conn)
	if err := h.app.Storage.SaveState(st); err != nil {
		h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
		return
	}
	h.successToast(w, "Conexão criada com sucesso")
}

func (h *Handler) EditDBConnectionDrawer(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, _ := h.app.Storage.LoadState()
	for _, c := range st.DBConnections {
		if c.ID == id {
			h.renderDrawer(w, "Editar Conexão", "database/connection-drawer.html", map[string]any{
				"Conn": c,
				"Keys": st.Keys,
			})
			return
		}
	}
	h.operationError(w, "Conexão não encontrada", http.StatusNotFound)
}

func (h *Handler) UpdateDBConnection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	for i, c := range st.DBConnections {
		if c.ID == id {
			updated := parseDBConnForm(r)
			updated.ID = c.ID
			updated.CreatedAt = c.CreatedAt
			st.DBConnections[i] = updated
			closeDBConn(id) // fecha conexão antiga do pool
			if err := h.app.Storage.SaveState(st); err != nil {
				h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
				return
			}
			h.successToast(w, "Conexão atualizada")
			return
		}
	}
	h.operationError(w, "Conexão não encontrada", http.StatusNotFound)
}

func (h *Handler) DeleteDBConnection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}
	closeDBConn(id)
	for i, c := range st.DBConnections {
		if c.ID == id {
			st.DBConnections = append(st.DBConnections[:i], st.DBConnections[i+1:]...)
			if err := h.app.Storage.SaveState(st); err != nil {
				h.operationError(w, "Erro ao salvar", http.StatusInternalServerError)
				return
			}
			h.successToast(w, "Conexão removida")
			return
		}
	}
	h.operationError(w, "Conexão não encontrada", http.StatusNotFound)
}

func (h *Handler) TestDBConnection(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, _ := h.app.Storage.LoadState()
	for _, c := range st.DBConnections {
		if c.ID == id {
			db, err := dbclient.Open(c)
			if err != nil {
				h.errorToast(w, "Falha: "+err.Error())
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			db.Close()
			h.successToastOnly(w, "Conexão OK!")
			return
		}
	}
	h.operationError(w, "Conexão não encontrada", http.StatusNotFound)
}

// ── Browse: Tables ───────────────────────────────────────────────

func (h *Handler) DBListTables(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	st, _ := h.app.Storage.LoadState()
	var conn storage.DBConnection
	found := false
	for _, c := range st.DBConnections {
		if c.ID == id {
			conn = c
			found = true
			break
		}
	}
	if !found {
		h.operationError(w, "Conexão não encontrada", http.StatusNotFound)
		return
	}

	db, err := getDBConn(conn)
	if err != nil {
		h.operationError(w, "Erro ao conectar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tables, err := dbclient.ListTables(db, conn.Driver)
	if err != nil {
		h.operationError(w, "Erro ao listar tabelas: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, "database/tables.html", map[string]any{
		"Conn":   conn,
		"Tables": tables,
	})
}

// ── Browse: Describe Table ───────────────────────────────────────

func (h *Handler) DBDescribeTable(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	table := chi.URLParam(r, "table")
	st, _ := h.app.Storage.LoadState()
	var conn storage.DBConnection
	found := false
	for _, c := range st.DBConnections {
		if c.ID == id {
			conn = c
			found = true
			break
		}
	}
	if !found {
		h.operationError(w, "Conexão não encontrada", http.StatusNotFound)
		return
	}

	db, err := getDBConn(conn)
	if err != nil {
		h.operationError(w, "Erro ao conectar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	columns, err := dbclient.DescribeTable(db, conn.Driver, table)
	if err != nil {
		h.operationError(w, "Erro ao descrever tabela: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, "database/structure.html", map[string]any{
		"Conn":    conn,
		"Table":   table,
		"Columns": columns,
	})
}

// ── Query ────────────────────────────────────────────────────────

func (h *Handler) DBRunQuery(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	id := chi.URLParam(r, "id")
	query := strings.TrimSpace(r.FormValue("query"))
	if query == "" {
		h.errorToast(w, "Query vazia")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	st, _ := h.app.Storage.LoadState()
	var conn storage.DBConnection
	found := false
	for _, c := range st.DBConnections {
		if c.ID == id {
			conn = c
			found = true
			break
		}
	}
	if !found {
		h.operationError(w, "Conexão não encontrada", http.StatusNotFound)
		return
	}

	db, err := getDBConn(conn)
	if err != nil {
		h.operationError(w, "Erro ao conectar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	result := dbclient.RunQuery(db, query)
	h.render(w, "database/results.html", map[string]any{
		"Conn":   conn,
		"Query":  query,
		"Result": result,
	})
}

// ── Helpers ──────────────────────────────────────────────────────

func parseDBConnForm(r *http.Request) storage.DBConnection {
	port, _ := strconv.Atoi(r.FormValue("port"))
	return storage.DBConnection{
		Name:     strings.TrimSpace(r.FormValue("name")),
		Driver:   r.FormValue("driver"),
		Host:     strings.TrimSpace(r.FormValue("host")),
		Port:     port,
		User:     strings.TrimSpace(r.FormValue("user")),
		Password: r.FormValue("password"),
		Database: strings.TrimSpace(r.FormValue("database")),
		SSLMode:  r.FormValue("sslMode"),
	}
}
