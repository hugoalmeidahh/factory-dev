package handler

import (
	"net/http"

	"github.com/seuusuario/factorydev/internal/app"
	"github.com/seuusuario/factorydev/internal/scanner"
)

// ScanSummary retorna um fragmento HTML com o resumo de recursos não gerenciados.
// Usado pelo sidebar via HTMX polling.
func (h *Handler) ScanSummary(w http.ResponseWriter, r *http.Request) {
	markHX(w, r)
	state, err := h.app.Storage.LoadState()
	if err != nil {
		h.operationError(w, app.FriendlyMessage(err), http.StatusInternalServerError)
		return
	}
	result := scanner.Scan(h.app.Paths, state)
	h.render(w, "partials/scan-badge.html", result)
}
