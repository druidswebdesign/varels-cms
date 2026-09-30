package inventory

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
	"github.com/druidswebdesign/varels-cms/internal/db/types"
	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/middleware"
	"github.com/druidswebdesign/varels-cms/internal/reporting"
	"github.com/druidswebdesign/varels-cms/internal/service"
	"github.com/druidswebdesign/varels-cms/internal/views/pages"
)

const holdListLimit = 200

// HandleHoldsList renders GET /holds. Expired holds are swept on load, then the
// list is shown (optionally filtered by status).
func (s *Server) HandleHoldsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = s.Q.ExpireStockHolds(ctx, sql.NullString{String: service.NowUTC(), Valid: true})

	var status interface{}
	if v := strings.TrimSpace(r.URL.Query().Get("status")); v != "" {
		status = v
	}
	holds, err := s.Q.ListStockHolds(ctx, sqlc.ListStockHoldsParams{
		Status: status,
		Limit:  holdListLimit,
		Offset: 0,
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	variants, err := s.Q.ListSellableVariants(ctx)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	locations, err := s.Q.ListLocations(ctx, 1)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.Holds(holds, variants, locations, middleware.CSRFToken(r)))
}

// HandleHoldCreate handles POST /holds.
func (s *Server) HandleHoldCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	variantID, vOK := common.ParseInt64(r.FormValue("variant_id"))
	locationID, lOK := common.ParseInt64(r.FormValue("location_id"))
	quantity, qOK := common.ParseInt64(r.FormValue("quantity"))
	if !vOK || !lOK || !qOK || quantity <= 0 {
		s.SetFlash(r, "Hold needs a variant, location and positive quantity.", "error")
		http.Redirect(w, r, "/holds", http.StatusSeeOther)
		return
	}

	source := types.HoldSource(strings.TrimSpace(r.FormValue("source")))
	switch source {
	case types.HoldSourceCart, types.HoldSourcePreorder, types.HoldSourceManual:
	default:
		source = types.HoldSourceManual
	}

	expires := common.NullString(r.FormValue("expires_at"))
	if expires.Valid {
		if t, err := time.ParseInLocation("2006-01-02T15:04", expires.String, reporting.ART); err == nil {
			expires = sql.NullString{String: reporting.ISOUTC(t), Valid: true}
		}
	}

	if _, err := s.Q.CreateStockHold(ctx, sqlc.CreateStockHoldParams{
		VariantID:  variantID,
		LocationID: locationID,
		Quantity:   quantity,
		Source:     source,
		RefID:      common.NullString(r.FormValue("ref_id")),
		Status:     types.HoldStatusActive,
		ExpiresAt:  expires,
	}); err != nil {
		common.ServerError(w, err)
		return
	}
	s.Audit(r, "create", "stock_hold", variantID, "hold")
	s.SetFlash(r, "Hold created.", "success")
	http.Redirect(w, r, "/holds", http.StatusSeeOther)
}

// HandleHoldRelease handles POST /holds/{id}/release.
func (s *Server) HandleHoldRelease(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.Q.GetStockHold(ctx, id); err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	if err := s.Q.SetStockHoldStatus(ctx, sqlc.SetStockHoldStatusParams{
		Status: types.HoldStatusReleased,
		ID:     id,
	}); err != nil {
		common.ServerError(w, err)
		return
	}
	s.Audit(r, "release", "stock_hold", id, "hold released")
	s.SetFlash(r, "Hold released.", "warning")
	http.Redirect(w, r, "/holds", http.StatusSeeOther)
}
