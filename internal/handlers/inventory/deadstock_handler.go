package inventory

import (
	"net/http"

	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/middleware"
	"github.com/druidswebdesign/varels-cms/internal/reporting"
	"github.com/druidswebdesign/varels-cms/internal/views/pages"
	"github.com/druidswebdesign/varels-cms/internal/views/partials"
)

const defaultStaleDays = 60

// HandleDeadstock renders the deadstock / trapped-cash page (admin+).
func (s *Server) HandleDeadstock(w http.ResponseWriter, r *http.Request) {
	days := staleDays(r)
	rows, err := s.Q.Deadstock(r.Context(), reporting.StaleSince(days))
	if err != nil {
		common.ServerError(w, err)
		return
	}
	var total int64
	for _, row := range rows {
		total += row.TrappedCashMinor
	}
	common.Render(w, r, http.StatusOK, pages.Deadstock(rows, total, days, middleware.CSRFToken(r)))
}

// HandleDeadstockTable renders the deadstock fragment for HTMX swaps.
func (s *Server) HandleDeadstockTable(w http.ResponseWriter, r *http.Request) {
	days := staleDays(r)
	rows, err := s.Q.Deadstock(r.Context(), reporting.StaleSince(days))
	if err != nil {
		common.ServerError(w, err)
		return
	}
	var total int64
	for _, row := range rows {
		total += row.TrappedCashMinor
	}
	common.Render(w, r, http.StatusOK, partials.DeadstockTable(rows, total, days))
}

func staleDays(r *http.Request) int {
	if v, ok := common.ParseInt64(r.URL.Query().Get("stale_days")); ok && v > 0 {
		return int(v)
	}
	return defaultStaleDays
}
