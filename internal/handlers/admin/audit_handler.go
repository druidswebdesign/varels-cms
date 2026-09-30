package admin

import (
	"net/http"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/middleware"
	"github.com/druidswebdesign/varels-cms/internal/views/pages"
)

const auditListLimit = 200

// HandleAuditLog renders GET /audit, the append-only audit trail (ADR-0010).
func (s *Server) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	logs, err := s.Q.ListAuditLogs(ctx, sqlc.ListAuditLogsParams{
		UserID:     common.NullInt64(q.Get("user")),
		EntityType: common.NullString(q.Get("entity")),
		FromTs:     common.NullString(q.Get("from")),
		ToTs:       common.NullString(q.Get("to")),
		Limit:      auditListLimit,
		Offset:     0,
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}

	common.Render(w, r, http.StatusOK, pages.Audit(logs,
		q.Get("user"), q.Get("entity"), q.Get("from"), q.Get("to"), middleware.CSRFToken(r)))
}
