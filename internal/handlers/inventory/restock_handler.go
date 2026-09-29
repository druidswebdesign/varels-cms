package inventory

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/db/types"
	"github.com/yourname/varels_cms/internal/handlers/common"
	"github.com/yourname/varels_cms/internal/handlers/middleware"
	"github.com/yourname/varels_cms/internal/repository"
	"github.com/yourname/varels_cms/internal/service"
	"github.com/yourname/varels_cms/internal/views/pages"
	"github.com/yourname/varels_cms/internal/views/partials"
)

const movementListLimit = 100

// HandleRestockForm renders the restock form for one variant.
func (s *Server) HandleRestockForm(w http.ResponseWriter, r *http.Request) {
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
	productName := ""
	if p, err := s.Q.GetProduct(ctx, v.ProductID); err == nil {
		productName = p.Name
	}
	locations, err := s.Q.ListLocations(ctx, 1)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	reasonCodes, err := s.Q.ListReasonCodes(ctx, types.ReasonKindStockAdjustment)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.RestockForm(v, productName, locations, reasonCodes, middleware.CSRFToken(r), r.URL.Query().Get("err")))
}

// HandleRestock handles POST /variants/{id}/restock: writes a positive stock
// movement and a FIFO cost layer, updating the variant cost when it changed
// (ADR-0012).
func (s *Server) HandleRestock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	qty, ok := common.ParseInt64(r.FormValue("quantity"))
	cost, costErr := common.ParsePesos(r.FormValue("cost"))
	locationID, locOK := common.ParseInt64(r.FormValue("location_id"))
	if !ok || qty <= 0 || costErr != nil || !locOK {
		http.Redirect(w, r, "/variants/"+common.Itoa(id)+"/restock?err=invalid", http.StatusSeeOther)
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	user := common.CurrentUserID(r)

	err = repository.WithTx(ctx, s.DB, s.Q, func(q *sqlc.Queries) error {
		if _, err := q.CreateStockMovement(ctx, sqlc.CreateStockMovementParams{
			VariantID:     id,
			LocationID:    locationID,
			State:         types.StockStateSellable,
			QuantityDelta: qty,
			RefType:       types.StockRefPOReceipt,
			Note:          sql.NullString{String: note, Valid: note != ""},
			CreatedBy:     user,
		}); err != nil {
			return err
		}
		if err := service.AddCostLayer(ctx, q, id, qty, cost, "restock"); err != nil {
			return err
		}

		v, err := q.GetVariant(ctx, id)
		if err != nil {
			return err
		}
		if v.CostMinor != cost {
			if _, err := q.CreateCostHistory(ctx, sqlc.CreateCostHistoryParams{
				VariantID:     id,
				CostMinor:     cost,
				EffectiveFrom: service.NowUTC(),
				Source:        types.CostSourceManual,
				Note:          sql.NullString{String: "restock", Valid: true},
				CreatedBy:     user,
			}); err != nil {
				return err
			}
		}
		return q.UpdateVariant(ctx, sqlc.UpdateVariantParams{
			Sku:                 v.Sku,
			Size:                v.Size,
			Color:               v.Color,
			Barcode:             v.Barcode,
			CostMinor:           cost,
			RetailPriceMinor:    v.RetailPriceMinor,
			WholesalePriceMinor: v.WholesalePriceMinor,
			ID:                  id,
		})
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}

	s.Audit(r, "restock", "product_variant", id, "restocked "+common.Itoa(qty)+" units")
	http.Redirect(w, r, "/products/"+common.Itoa(productIDFor(r, s, id)), http.StatusSeeOther)
}

// HandleAdjustStock handles POST /variants/{id}/adjust. A reason code is
// required and the sign of quantity_delta decides direction (ADR-0017).
func (s *Server) HandleAdjustStock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	qty, ok := common.ParseInt64(r.FormValue("quantity"))
	locationID, locOK := common.ParseInt64(r.FormValue("location_id"))
	reasonCode := strings.TrimSpace(r.FormValue("reason_code"))
	state := types.StockState(strings.TrimSpace(r.FormValue("state")))
	if state == "" {
		state = types.StockStateSellable
	}
	if !ok || qty == 0 || !locOK || reasonCode == "" {
		http.Redirect(w, r, "/products/"+common.Itoa(productIDFor(r, s, id))+"?err=adjust", http.StatusSeeOther)
		return
	}

	if _, err := s.Q.CreateStockMovement(ctx, sqlc.CreateStockMovementParams{
		VariantID:     id,
		LocationID:    locationID,
		State:         state,
		QuantityDelta: qty,
		ReasonCode:    sql.NullString{String: reasonCode, Valid: true},
		RefType:       types.StockRefAdjustment,
		Note:          sql.NullString{String: strings.TrimSpace(r.FormValue("note")), Valid: true},
		CreatedBy:     common.CurrentUserID(r),
	}); err != nil {
		common.ServerError(w, err)
		return
	}

	s.Audit(r, "adjust", "product_variant", id, reasonCode)
	http.Redirect(w, r, "/products/"+common.Itoa(productIDFor(r, s, id)), http.StatusSeeOther)
}

// HandleRestocksList renders the restock history.
func (s *Server) HandleRestocksList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	movements, err := s.Q.ListRestocks(ctx, sqlc.ListRestocksParams{Limit: movementListLimit, Offset: 0})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	skus, _ := s.variantSkus(ctx)
	locations, _ := repository.LocationNames(ctx, s.Q)
	common.Render(w, r, http.StatusOK, pages.RestockList(movements, skus, locations, middleware.CSRFToken(r)))
}

// HandleStockMovements renders the append-only ledger with filters.
func (s *Server) HandleStockMovements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	var variantID, locationID, refType interface{}
	if v, ok := common.ParseInt64(query.Get("variant")); ok {
		variantID = v
	}
	if v, ok := common.ParseInt64(query.Get("location")); ok {
		locationID = v
	}
	if v := strings.TrimSpace(query.Get("ref_type")); v != "" {
		refType = v
	}

	movements, err := s.Q.ListStockMovements(ctx, sqlc.ListStockMovementsParams{
		VariantID:  variantID,
		LocationID: locationID,
		RefType:    refType,
		FromTs:     nil,
		ToTs:       nil,
		Limit:      movementListLimit,
		Offset:     0,
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	skus, _ := s.variantSkus(ctx)
	locations, _ := repository.LocationNames(ctx, s.Q)
	common.Render(w, r, http.StatusOK, pages.StockMovements(movements, skus, locations, middleware.CSRFToken(r)))
}

// HandleLowStockWidget renders GET /dashboard/low-stock as an HTMX fragment.
func (s *Server) HandleLowStockWidget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.Q.ListLowStock(ctx)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	skus, thresholds := s.variantSkusAndThresholds(ctx)
	common.Render(w, r, http.StatusOK, partials.LowStockWidget(rows, skus, thresholds))
}

// variantSkus maps variant id -> "product · sku".
func (s *Server) variantSkus(ctx context.Context) (map[int64]string, error) {
	variants, err := s.Q.ListSellableVariants(ctx)
	if err != nil {
		return map[int64]string{}, err
	}
	out := make(map[int64]string, len(variants))
	for _, v := range variants {
		out[v.ID] = v.ProductName + " · " + v.Sku
	}
	return out, nil
}

func (s *Server) variantSkusAndThresholds(ctx context.Context) (map[int64]string, map[int64]int64) {
	variants, err := s.Q.ListSellableVariants(ctx)
	if err != nil {
		return map[int64]string{}, map[int64]int64{}
	}
	skus := make(map[int64]string, len(variants))
	thresholds := make(map[int64]int64, len(variants))
	for _, v := range variants {
		skus[v.ID] = v.ProductName + " · " + v.Sku
		thresholds[v.ID] = v.LowStockThreshold
	}
	return skus, thresholds
}

// productIDFor returns the parent product id for redirects.
func productIDFor(r *http.Request, s *Server, variantID int64) int64 {
	if v, err := s.Q.GetVariant(r.Context(), variantID); err == nil {
		return v.ProductID
	}
	return 0
}
