package admin

import (
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
)

const (
	supplierListLimit = 200
	poListLimit       = 200
)

// HandleSuppliersList renders GET /suppliers.
func (s *Server) HandleSuppliersList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	suppliers, err := s.Q.ListSuppliers(ctx, 1)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.Suppliers(suppliers, middleware.CSRFToken(r)))
}

// HandleSupplierCreate handles POST /suppliers.
func (s *Server) HandleSupplierCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.SetFlash(r, "Supplier name is required.", "error")
		http.Redirect(w, r, "/suppliers", http.StatusSeeOther)
		return
	}
	if _, err := s.Q.CreateSupplier(ctx, sqlc.CreateSupplierParams{
		Name:         name,
		ContactName:  common.NullString(r.FormValue("contact_name")),
		Email:        common.NullString(r.FormValue("email")),
		Phone:        common.NullString(r.FormValue("phone")),
		LeadTimeDays: common.NullInt64(r.FormValue("lead_time_days")),
		Terms:        common.NullString(r.FormValue("terms")),
		Notes:        common.NullString(r.FormValue("notes")),
	}); err != nil {
		if common.IsUniqueViolation(err) {
			s.SetFlash(r, "That supplier already exists.", "error")
			http.Redirect(w, r, "/suppliers", http.StatusSeeOther)
			return
		}
		common.ServerError(w, err)
		return
	}
	s.Audit(r, "create", "supplier", 0, "supplier")
	s.SetFlash(r, "Supplier added.", "success")
	http.Redirect(w, r, "/suppliers", http.StatusSeeOther)
}

// HandleSupplierUpdate handles POST /suppliers/{id}.
func (s *Server) HandleSupplierUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.SetFlash(r, "Supplier name is required.", "error")
		http.Redirect(w, r, "/suppliers", http.StatusSeeOther)
		return
	}
	isActive := int64(1)
	if r.FormValue("is_active") == "0" {
		isActive = 0
	}
	if err := s.Q.UpdateSupplier(ctx, sqlc.UpdateSupplierParams{
		Name:         name,
		ContactName:  common.NullString(r.FormValue("contact_name")),
		Email:        common.NullString(r.FormValue("email")),
		Phone:        common.NullString(r.FormValue("phone")),
		LeadTimeDays: common.NullInt64(r.FormValue("lead_time_days")),
		Terms:        common.NullString(r.FormValue("terms")),
		Notes:        common.NullString(r.FormValue("notes")),
		IsActive:     isActive,
		ID:           id,
	}); err != nil {
		common.ServerError(w, err)
		return
	}
	s.SetFlash(r, "Supplier updated.", "success")
	http.Redirect(w, r, "/suppliers", http.StatusSeeOther)
}

// HandlePurchaseOrdersList renders GET /purchase-orders.
func (s *Server) HandlePurchaseOrdersList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	var supplierID, status interface{}
	if v, ok := common.ParseInt64(query.Get("supplier")); ok {
		supplierID = v
	}
	if v := strings.TrimSpace(query.Get("status")); v != "" {
		status = v
	}

	orders, err := s.Q.ListPurchaseOrders(ctx, sqlc.ListPurchaseOrdersParams{
		SupplierID: supplierID,
		Status:     status,
		Limit:      poListLimit,
		Offset:     0,
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	suppliers, err := s.Q.ListSuppliers(ctx, 1)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	locations, err := s.Q.ListAllLocations(ctx)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.PurchaseOrders(orders, suppliers, locations, middleware.CSRFToken(r)))
}

// HandlePurchaseOrderCreate handles POST /purchase-orders.
func (s *Server) HandlePurchaseOrderCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	supplierID, sOK := common.ParseInt64(r.FormValue("supplier_id"))
	locationID, lOK := common.ParseInt64(r.FormValue("location_id"))
	if !sOK || !lOK {
		s.SetFlash(r, "Pick a supplier and a receive location.", "error")
		http.Redirect(w, r, "/purchase-orders", http.StatusSeeOther)
		return
	}

	po, err := s.Q.CreatePurchaseOrder(ctx, sqlc.CreatePurchaseOrderParams{
		SupplierID: supplierID,
		LocationID: locationID,
		Status:     types.PurchaseOrderStatusDraft,
		OrderedAt:  sql.NullString{String: service.NowUTC(), Valid: true},
		ExpectedAt: common.NullString(r.FormValue("expected_at")),
		Notes:      common.NullString(r.FormValue("notes")),
		CreatedBy:  common.CurrentUserID(r),
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	s.Audit(r, "create", "purchase_order", po.ID, "purchase order")
	s.SetFlash(r, "Purchase order created.", "success")
	http.Redirect(w, r, "/purchase-orders/"+common.Itoa(po.ID), http.StatusSeeOther)
}

// HandlePurchaseOrderDetail renders GET /purchase-orders/{id}.
func (s *Server) HandlePurchaseOrderDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	po, err := s.Q.GetPurchaseOrder(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	items, err := s.Q.ListPurchaseOrderItems(ctx, id)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	variants, err := s.Q.ListSellableVariants(ctx)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	supplier := ""
	if sup, err := s.Q.GetSupplier(ctx, po.SupplierID); err == nil {
		supplier = sup.Name
	}
	location := ""
	if loc, err := s.Q.GetLocation(ctx, po.LocationID); err == nil {
		location = loc.Name
	}
	common.Render(w, r, http.StatusOK, pages.PurchaseOrderDetail(po, items, variants, supplier, location, middleware.CSRFToken(r)))
}

// HandlePurchaseOrderAddItem handles POST /purchase-orders/{id}/items.
func (s *Server) HandlePurchaseOrderAddItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	variantID, vOK := common.ParseInt64(r.FormValue("variant_id"))
	qty, qOK := common.ParseInt64(r.FormValue("quantity"))
	cost, cErr := common.ParsePesos(r.FormValue("unit_cost"))
	if !vOK || !qOK || qty <= 0 || cErr != nil || cost < 0 {
		s.SetFlash(r, "Add a variant, a positive quantity and a unit cost.", "error")
		http.Redirect(w, r, "/purchase-orders/"+common.Itoa(id), http.StatusSeeOther)
		return
	}
	if err := s.Q.UpsertPurchaseOrderItem(ctx, sqlc.UpsertPurchaseOrderItemParams{
		PurchaseOrderID: id,
		VariantID:       variantID,
		QuantityOrdered: qty,
		UnitCostMinor:   cost,
		LineTotalMinor:  qty * cost,
	}); err != nil {
		common.ServerError(w, err)
		return
	}
	s.SetFlash(r, "Line added.", "success")
	http.Redirect(w, r, "/purchase-orders/"+common.Itoa(id), http.StatusSeeOther)
}

// HandlePurchaseOrderStatus handles POST /purchase-orders/{id}/status.
func (s *Server) HandlePurchaseOrderStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	status := types.PurchaseOrderStatus(strings.TrimSpace(r.FormValue("status")))
	switch status {
	case types.PurchaseOrderStatusDraft, types.PurchaseOrderStatusOrdered,
		types.PurchaseOrderStatusPartial, types.PurchaseOrderStatusReceived,
		types.PurchaseOrderStatusCancelled:
	default:
		s.SetFlash(r, "Invalid status.", "error")
		http.Redirect(w, r, "/purchase-orders/"+common.Itoa(id), http.StatusSeeOther)
		return
	}

	receivedAt := sql.NullString{}
	if status == types.PurchaseOrderStatusReceived {
		receivedAt = sql.NullString{String: service.NowUTC(), Valid: true}
	}
	if err := s.Q.SetPurchaseOrderStatus(ctx, sqlc.SetPurchaseOrderStatusParams{
		Status:     status,
		ReceivedAt: receivedAt,
		ID:         id,
	}); err != nil {
		common.ServerError(w, err)
		return
	}
	s.Audit(r, "status", "purchase_order", id, string(status))
	s.SetFlash(r, "PO status updated.", "success")
	http.Redirect(w, r, "/purchase-orders/"+common.Itoa(id), http.StatusSeeOther)
}

// HandlePurchaseOrderReceive handles POST /purchase-orders/{id}/receive. Each
// received line writes a positive stock movement (ref_type po_receipt) and a FIFO
// cost layer, then advances the PO status, all in one transaction (ADR-0012).
func (s *Server) HandlePurchaseOrderReceive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	po, err := s.Q.GetPurchaseOrder(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	_ = r.ParseForm()

	items, err := s.Q.ListPurchaseOrderItems(ctx, id)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	byID := make(map[int64]sqlc.PurchaseOrderItem, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}

	receiveQty := map[int64]int64{}
	itemIDs := r.Form["item_id"]
	qtys := r.Form["qty"]
	for i, rawID := range itemIDs {
		if i >= len(qtys) {
			break
		}
		itemID, ok := common.ParseInt64(rawID)
		if !ok {
			continue
		}
		qty, ok := common.ParseInt64(qtys[i])
		if !ok || qty <= 0 {
			continue
		}
		receiveQty[itemID] = qty
	}
	if len(receiveQty) == 0 {
		s.SetFlash(r, "Enter a quantity to receive.", "error")
		http.Redirect(w, r, "/purchase-orders/"+common.Itoa(id), http.StatusSeeOther)
		return
	}

	err = repository.WithTx(ctx, s.DB, s.Q, func(q *sqlc.Queries) error {
		for itemID, qty := range receiveQty {
			item, ok := byID[itemID]
			if !ok {
				continue
			}
			if _, err := q.CreateStockMovement(ctx, sqlc.CreateStockMovementParams{
				VariantID:     item.VariantID,
				LocationID:    po.LocationID,
				State:         types.StockStateSellable,
				QuantityDelta: qty,
				RefType:       types.StockRefPOReceipt,
				RefID:         sql.NullInt64{Int64: po.ID, Valid: true},
				CreatedBy:     common.CurrentUserID(r),
			}); err != nil {
				return err
			}
			if err := service.AddCostLayer(ctx, q, item.VariantID, qty, item.UnitCostMinor, "po_receipt"); err != nil {
				return err
			}
			if err := q.ReceivePurchaseOrderItem(ctx, sqlc.ReceivePurchaseOrderItemParams{Qty: qty, ID: itemID}); err != nil {
				return err
			}
		}

		fresh, err := q.ListPurchaseOrderItems(ctx, po.ID)
		if err != nil {
			return err
		}
		allReceived := true
		for _, it := range fresh {
			if it.QuantityReceived < it.QuantityOrdered {
				allReceived = false
				break
			}
		}
		status := types.PurchaseOrderStatusPartial
		receivedAt := sql.NullString{}
		if allReceived {
			status = types.PurchaseOrderStatusReceived
			receivedAt = sql.NullString{String: service.NowUTC(), Valid: true}
		}
		return q.SetPurchaseOrderStatus(ctx, sqlc.SetPurchaseOrderStatusParams{
			Status:     status,
			ReceivedAt: receivedAt,
			ID:         po.ID,
		})
	})
	if err != nil {
		common.ServerError(w, err)
		return
	}
	s.Audit(r, "receive", "purchase_order", id, "po receipt")
	s.SetFlash(r, "Stock received.", "success")
	http.Redirect(w, r, "/purchase-orders/"+common.Itoa(id), http.StatusSeeOther)
}
