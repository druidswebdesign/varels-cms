package handlers

import (
	"net/http"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/views/pages"
)

const auditListLimit = 200

// HandleAuditLog renders GET /audit, the append-only audit trail (ADR-0010).
func (s *Server) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	logs, err := s.Q.ListAuditLogs(ctx, sqlc.ListAuditLogsParams{
		UserID:     nullInt64(q.Get("user")),
		EntityType: nullString(q.Get("entity")),
		FromTs:     nullString(q.Get("from")),
		ToTs:       nullString(q.Get("to")),
		Limit:      auditListLimit,
		Offset:     0,
	})
	if err != nil {
		serverError(w, err)
		return
	}

	render(w, r, http.StatusOK, pages.Audit(logs,
		q.Get("user"), q.Get("entity"), q.Get("from"), q.Get("to"), CSRFToken(r)))
}
