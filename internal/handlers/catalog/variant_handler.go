package catalog

import (
	"net/http"
	"strconv"
	"strings"

	"context"
	"database/sql"

	"github.com/a-h/templ"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
	"github.com/druidswebdesign/varels-cms/internal/db/types"
	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/middleware"
	"github.com/druidswebdesign/varels-cms/internal/repository"
	"github.com/druidswebdesign/varels-cms/internal/views/flash"
	"github.com/druidswebdesign/varels-cms/internal/views/pages"
	"github.com/druidswebdesign/varels-cms/internal/views/partials"
)

// sellableTotals maps variant id to its total sellable quantity for a product.
func (s *Server) sellableTotals(ctx context.Context, productID int64) map[int64]int64 {
	rows, err := s.Q.ListSellableTotalsByProduct(ctx, productID)
	if err != nil {
		return map[int64]int64{}
	}
	out := make(map[int64]int64, len(rows))
	for _, r := range rows {
		out[r.VariantID] = r.OnHand
	}
	return out
}

// HandleVariantTable renders GET /products/{id}/variants as an HTMX fragment.
func (s *Server) HandleVariantTable(w http.ResponseWriter, r *http.Request) {
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	variants, err := s.Q.ListVariantsByProduct(r.Context(), sqlc.ListVariantsByProductParams{ProductID: id, IsArchived: 0})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, partials.VariantTable(id, variants, s.sellableTotals(r.Context(), id), middleware.CSRFToken(r)))
}

// HandleVariantCreate handles POST /products/{id}/variants.
func (s *Server) HandleVariantCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	productID, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	sku := strings.TrimSpace(r.FormValue("sku"))
	if sku == "" {
		http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
		return
	}

	cost, err := common.ParsePesos(r.FormValue("cost"))
	if err != nil {
		http.Redirect(w, r, "/products/"+common.Itoa(productID)+"?err=cost", http.StatusSeeOther)
		return
	}
	retail, err := common.ParsePesos(r.FormValue("retail"))
	if err != nil {
		http.Redirect(w, r, "/products/"+common.Itoa(productID)+"?err=retail", http.StatusSeeOther)
		return
	}
	wholesaleRaw := strings.TrimSpace(r.FormValue("wholesale"))
	wholesale, err := common.ParsePesos(wholesaleRaw)
	if err != nil {
		http.Redirect(w, r, "/products/"+common.Itoa(productID)+"?err=wholesale", http.StatusSeeOther)
		return
	}

	var threshold int64
	if v := strings.TrimSpace(r.FormValue("threshold")); v != "" {
		threshold, _ = strconv.ParseInt(v, 10, 64)
	}

	if _, err := s.Q.CreateVariant(ctx, sqlc.CreateVariantParams{
		ProductID:           productID,
		Sku:                 sku,
		Size:                common.NullString(r.FormValue("size")),
		Color:               common.NullString(r.FormValue("color")),
		Barcode:             common.NullString(r.FormValue("barcode")),
		CostMinor:           cost,
		RetailPriceMinor:    retail,
		WholesalePriceMinor: sql.NullInt64{Int64: wholesale, Valid: wholesaleRaw != ""},
		LowStockThreshold:   threshold,
	}); err != nil {
		if common.IsUniqueViolation(err) {
			s.SetFlash(r, "That SKU is already in use.", "error")
			if common.IsHX(r) {
				s.renderVariantTable(w, r, productID, "That SKU is already in use.", "error")
				return
			}
			http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
			return
		}
		common.ServerError(w, err)
		return
	}

	s.SetFlash(r, "Variant added.", "success")
	if common.IsHX(r) {
		s.renderVariantTable(w, r, productID, "Variant added.", "success")
		return
	}
	http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
}

// renderVariantTable re-renders the variant fragment plus an out-of-band flash
// for HTMX mutation responses.
func (s *Server) renderVariantTable(w http.ResponseWriter, r *http.Request, productID int64, message, tone string) {
	variants, err := s.Q.ListVariantsByProduct(r.Context(), sqlc.ListVariantsByProductParams{ProductID: productID, IsArchived: 0})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, templ.Join(
		partials.VariantTable(productID, variants, s.sellableTotals(r.Context(), productID), middleware.CSRFToken(r)),
		partials.FlashOOB(flash.Flash{Message: message, Tone: tone}),
	))
}

// HandleVariantForm renders GET /variants/{id}/edit.
func (s *Server) HandleVariantForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.VariantForm(v, middleware.CSRFToken(r)))
}

// HandleVariantUpdate handles POST /variants/{id}.
func (s *Server) HandleVariantUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	_ = r.ParseForm()

	sku := strings.TrimSpace(r.FormValue("sku"))
	if sku == "" {
		http.Redirect(w, r, "/variants/"+common.Itoa(id)+"/edit?err=sku", http.StatusSeeOther)
		return
	}
	cost, costErr := common.ParsePesos(r.FormValue("cost"))
	retail, retailErr := common.ParsePesos(r.FormValue("retail"))
	wholesaleRaw := strings.TrimSpace(r.FormValue("wholesale"))
	wholesale, wholesaleErr := common.ParsePesos(wholesaleRaw)
	if costErr != nil || retailErr != nil || wholesaleErr != nil {
		http.Redirect(w, r, "/variants/"+common.Itoa(id)+"/edit?err=price", http.StatusSeeOther)
		return
	}

	if err := s.Q.UpdateVariant(ctx, sqlc.UpdateVariantParams{
		Sku:                 sku,
		Size:                common.NullString(r.FormValue("size")),
		Color:               common.NullString(r.FormValue("color")),
		Barcode:             common.NullString(r.FormValue("barcode")),
		CostMinor:           cost,
		RetailPriceMinor:    retail,
		WholesalePriceMinor: sql.NullInt64{Int64: wholesale, Valid: wholesaleRaw != ""},
		ID:                  id,
	}); err != nil {
		if common.IsUniqueViolation(err) {
			http.Redirect(w, r, "/variants/"+common.Itoa(id)+"/edit?err=duplicate", http.StatusSeeOther)
			return
		}
		common.ServerError(w, err)
		return
	}

	http.Redirect(w, r, "/products/"+common.Itoa(v.ProductID), http.StatusSeeOther)
}

// HandleVariantArchive handles POST /variants/{id}/archive.
func (s *Server) HandleVariantArchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	if err := s.Q.SetVariantArchived(ctx, sqlc.SetVariantArchivedParams{IsArchived: 1, ID: id}); err != nil {
		common.ServerError(w, err)
		return
	}
	if common.IsHX(r) {
		s.renderVariantTable(w, r, v.ProductID, "Variant archived.", "warning")
		return
	}
	s.SetFlash(r, "Variant archived.", "warning")
	http.Redirect(w, r, "/products/"+common.Itoa(v.ProductID), http.StatusSeeOther)
}

// HandleVariantThreshold handles POST /variants/{id}/threshold.
func (s *Server) HandleVariantThreshold(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	threshold, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("threshold")), 10, 64)
	if err := s.Q.SetVariantThreshold(ctx, sqlc.SetVariantThresholdParams{LowStockThreshold: threshold, ID: id}); err != nil {
		common.ServerError(w, err)
		return
	}
	if common.IsHX(r) {
		s.renderVariantTable(w, r, v.ProductID, "Threshold updated.", "success")
		return
	}
	s.SetFlash(r, "Threshold updated.", "success")
	http.Redirect(w, r, "/products/"+common.Itoa(v.ProductID), http.StatusSeeOther)
}

// HandleVariantSetQuantity handles POST /variants/{id}/quantity: sets the total
// sellable quantity to an absolute value. Stock is append-only (ADR-0017), so it
// records "recount" adjustment movements for the difference rather than
// overwriting stock_levels. A reduction is drawn down across the sellable stock
// levels so no location goes negative.
func (s *Server) HandleVariantSetQuantity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := s.Q.GetVariant(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	_ = r.ParseForm()

	target, ok := common.ParseInt64(r.FormValue("quantity"))
	if !ok || target < 0 {
		s.variantQuantityResult(w, r, v.ProductID, "Quantity must be zero or more.", "error")
		return
	}

	levels, err := s.Q.ListStockByVariant(ctx, id)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	var current int64
	for _, l := range levels {
		if l.State == types.StockStateSellable {
			current += l.Quantity
		}
	}
	delta := target - current
	if delta == 0 {
		s.variantQuantityResult(w, r, v.ProductID, "Quantity unchanged.", "success")
		return
	}

	// Increase lands on the sellable level holding the most stock, else any
	// existing level, else the first active location.
	locationID := int64(0)
	best := int64(-1)
	for _, l := range levels {
		if l.State == types.StockStateSellable && l.Quantity > best {
			best, locationID = l.Quantity, l.LocationID
		}
	}
	if locationID == 0 && len(levels) > 0 {
		locationID = levels[0].LocationID
	}
	if locationID == 0 {
		if locs, err := s.Q.ListLocations(ctx, 1); err == nil && len(locs) > 0 {
			locationID = locs[0].ID
		}
	}
	if locationID == 0 {
		s.variantQuantityResult(w, r, v.ProductID, "No location available to hold stock.", "error")
		return
	}

	user := common.CurrentUserID(r)
	err = repository.WithTx(ctx, s.DB, s.Q, func(q *sqlc.Queries) error {
		if delta > 0 {
			_, err := q.CreateStockMovement(ctx, recountMovement(id, locationID, delta, user))
			return err
		}
		remaining := -delta
		for _, l := range levels {
			if remaining == 0 {
				break
			}
			if l.State != types.StockStateSellable || l.Quantity <= 0 {
				continue
			}
			take := l.Quantity
			if take > remaining {
				take = remaining
			}
			if _, err := q.CreateStockMovement(ctx, recountMovement(id, l.LocationID, -take, user)); err != nil {
				return err
			}
			remaining -= take
		}
		return nil
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}

	s.Audit(r, "recount", "product_variant", id, "set sellable quantity to "+common.Itoa(target))
	s.variantQuantityResult(w, r, v.ProductID, "Quantity set to "+common.Itoa(target)+".", "success")
}

// recountMovement builds a sellable recount adjustment movement.
func recountMovement(variantID, locationID, delta int64, user sql.NullInt64) sqlc.CreateStockMovementParams {
	return sqlc.CreateStockMovementParams{
		VariantID:     variantID,
		LocationID:    locationID,
		State:         types.StockStateSellable,
		QuantityDelta: delta,
		ReasonCode:    sql.NullString{String: "recount", Valid: true},
		RefType:       types.StockRefAdjustment,
		Note:          sql.NullString{String: "recount", Valid: true},
		CreatedBy:     user,
	}
}

// variantQuantityResult re-renders the variant fragment for HTMX, or redirects
// to the product for a plain form post.
func (s *Server) variantQuantityResult(w http.ResponseWriter, r *http.Request, productID int64, message, tone string) {
	if common.IsHX(r) {
		s.renderVariantTable(w, r, productID, message, tone)
		return
	}
	s.SetFlash(r, message, tone)
	http.Redirect(w, r, "/products/"+common.Itoa(productID), http.StatusSeeOther)
}
