package common

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
)

// Audit writes an audit_logs row, logging but not failing on error. It is shared
// by every handler package.
func (s *Server) Audit(r *http.Request, action, entityType string, entityID int64, note string) {
	_, err := s.Q.CreateAuditLog(r.Context(), sqlc.CreateAuditLogParams{
		UserID:     CurrentUserID(r),
		Action:     action,
		EntityType: entityType,
		EntityID:   sql.NullInt64{Int64: entityID, Valid: true},
		Note:       sql.NullString{String: note, Valid: note != ""},
		Ip:         sql.NullString{String: ClientIP(r), Valid: true},
	})
	if err != nil {
		log.Printf("audit: %v", err)
	}
}
