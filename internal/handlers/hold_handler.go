package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/db/types"
	"github.com/yourname/varels_cms/internal/views/pages"
)

const holdListLimit = 200

// HandleHoldsList renders GET /holds. Expired holds are swept on load, then the
// list is shown (optionally filtered by status).
func (s *Server) HandleHoldsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = s.Q.ExpireStockHolds(ctx, sql.NullString{String: nowUTC(), Valid: true})

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
		serverError(w, err)
		return
	}
	variants, err := s.Q.ListSellableVariants(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	locations, err := s.Q.ListLocations(ctx, 1)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.Holds(holds, variants, locations, CSRFToken(r)))
}

// HandleHoldCreate handles POST /holds.
func (s *Server) HandleHoldCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	variantID, vOK := parseInt64(r.FormValue("variant_id"))
	locationID, lOK := parseInt64(r.FormValue("location_id"))
	quantity, qOK := parseInt64(r.FormValue("quantity"))
	if !vOK || !lOK || !qOK || quantity <= 0 {
		s.setFlash(r, "Hold needs a variant, location and positive quantity.", "error")
		http.Redirect(w, r, "/holds", http.StatusSeeOther)
		return
	}

	source := types.HoldSource(strings.TrimSpace(r.FormValue("source")))
	switch source {
	case types.HoldSourceCart, types.HoldSourcePreorder, types.HoldSourceManual:
	default:
		source = types.HoldSourceManual
	}

	expires := nullString(r.FormValue("expires_at"))
	if expires.Valid {
		if t, err := time.ParseInLocation("2006-01-02T15:04", expires.String, art); err == nil {
			expires = sql.NullString{String: isoUTC(t), Valid: true}
		}
	}

	if _, err := s.Q.CreateStockHold(ctx, sqlc.CreateStockHoldParams{
		VariantID:  variantID,
		LocationID: locationID,
		Quantity:   quantity,
		Source:     source,
		RefID:      nullString(r.FormValue("ref_id")),
		Status:     types.HoldStatusActive,
		ExpiresAt:  expires,
	}); err != nil {
		serverError(w, err)
		return
	}
	s.audit(r, "create", "stock_hold", variantID, "hold")
	s.setFlash(r, "Hold created.", "success")
	http.Redirect(w, r, "/holds", http.StatusSeeOther)
}

// HandleHoldRelease handles POST /holds/{id}/release.
func (s *Server) HandleHoldRelease(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := urlID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.Q.GetStockHold(ctx, id); err != nil {
		if isNotFound(err) {
			http.NotFound(w, r)
			return
		}
		serverError(w, err)
		return
	}
	if err := s.Q.SetStockHoldStatus(ctx, sqlc.SetStockHoldStatusParams{
		Status: types.HoldStatusReleased,
		ID:     id,
	}); err != nil {
		serverError(w, err)
		return
	}
	s.audit(r, "release", "stock_hold", id, "hold released")
	s.setFlash(r, "Hold released.", "warning")
	http.Redirect(w, r, "/holds", http.StatusSeeOther)
}
